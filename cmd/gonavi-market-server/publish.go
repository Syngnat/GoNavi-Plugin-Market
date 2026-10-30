package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"

	"github.com/Syngnat/GoNavi-Plugin-Market/internal/logger"
	"github.com/Syngnat/GoNavi-Plugin-Market/internal/market"
)

// officialPublisherAccount is the machine identity behind command-line official
// imports. The 19-digit placeholder can never collide with a real GitHub user id.
const officialPublisherAccount = "9999999999999999999"

// publishOfficial drives one zip through the ordinary submission pipeline plus an
// administrator approval, so an import keeps every invariant and audit entry.
// The reviewer must already hold the admin role; this command grants nothing
// beyond the dedicated official publisher machine account.
func publishOfficial(ctx context.Context, db *sql.DB, options commandOptions) error {
	if options.publishArchive == "" || options.reviewer == "" {
		return errors.New("official publish requires an archive and a reviewer github id")
	}
	repository := market.NewPostgres(db)
	if err := repository.GrantRole(ctx, officialPublisherAccount, options.namespace, "publisher", true); err != nil {
		return err
	}
	reviewer, err := repository.AccountByID(ctx, options.reviewer)
	if err != nil || reviewer.Role != "admin" {
		return errors.New("reviewer account must be granted the admin role first")
	}
	packageInfo, err := market.ValidatePackage(ctx, options.publishArchive)
	if err != nil {
		return err
	}
	if current, pluginErr := repository.Plugin(ctx, packageInfo.ID); pluginErr == nil &&
		current.Version == packageInfo.Version && current.Artifact != nil && current.Artifact.SHA256 == packageInfo.SHA256 {
		logger.Infof("market plugin %s %s already published with identical artifact", packageInfo.ID, packageInfo.Version)
		return trackOfficialSource(ctx, repository, packageInfo.ID, options)
	}
	publisher := market.Account{ID: officialPublisherAccount, Role: "publisher", PublisherID: options.namespace, Official: true}
	submission, err := repository.BeginSubmission(ctx, publisher, packageInfo.ID, packageInfo.Version)
	if err != nil {
		return err
	}
	for _, state := range []string{"submitted", "validating"} {
		if submission, err = repository.AdvanceSubmission(ctx, publisher, submission.ID, state, ""); err != nil {
			return err
		}
	}
	if err = storeOfficialArchive(ctx, options.publishArchive, packageInfo); err != nil {
		return err
	}
	if _, err = repository.ValidateSubmission(ctx, publisher, submission.ID, packageInfo); err != nil {
		return err
	}
	if _, err = repository.Review(ctx, reviewer, submission.ID, "approve", "official import"); err != nil {
		return err
	}
	logger.Infof("market plugin %s %s published by official import", packageInfo.ID, packageInfo.Version)
	return trackOfficialSource(ctx, repository, packageInfo.ID, options)
}

// trackOfficialSource registers where the plugin is released so the periodic
// sync can keep it current without another manual import.
func trackOfficialSource(ctx context.Context, repository *market.Postgres, pluginID string, options commandOptions) error {
	if options.officialSource == "" {
		return nil
	}
	if err := repository.SetPluginTracking(ctx, pluginID, options.officialSource, options.officialTagPrefix); err != nil {
		return fmt.Errorf("track official source failed: %w", err)
	}
	logger.Infof("market plugin %s tracks %s (tag prefix %q)", pluginID, options.officialSource, options.officialTagPrefix)
	return nil
}

func storeOfficialArchive(ctx context.Context, path string, packageInfo market.ValidatedPackage) error {
	store, err := configuredArtifacts()
	if err != nil {
		return err
	}
	file, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("open archive failed: %w", err)
	}
	defer file.Close()
	return store.Put(ctx, packageInfo.SHA256, file, packageInfo.SizeBytes)
}
