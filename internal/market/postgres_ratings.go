package market

import (
	"context"
	"database/sql"
	"strings"

	"github.com/lib/pq"
)

const ratingColumns = `id,user_id,plugin_id,parent_id,stars,body,version,created_at,updated_at`

func scanRating(row scanner) (Rating, error) {
	var r Rating
	var parent sql.NullInt64
	err := row.Scan(&r.ID, &r.UserID, &r.PluginID, &parent, &r.Stars, &r.Body, &r.Version, &r.CreatedAt, &r.UpdatedAt)
	if parent.Valid {
		id := parent.Int64
		r.ParentID = &id
	}
	return r, repositoryError(err)
}

// Ratings lists visible top-level reviews with their visible replies.
func (p *Postgres) Ratings(ctx context.Context, id string) ([]Rating, error) {
	if _, err := p.Plugin(ctx, id); err != nil {
		return nil, err
	}
	rows, err := p.db.QueryContext(ctx, `SELECT `+ratingColumns+` FROM market_ratings
 WHERE plugin_id=$1 AND parent_id IS NULL AND NOT hidden AND NOT deleted
 ORDER BY updated_at DESC,id DESC LIMIT 100`, id)
	if err != nil {
		return nil, repositoryError(err)
	}
	defer rows.Close()
	result := make([]Rating, 0)
	ids := make([]int64, 0)
	for rows.Next() {
		rating, scanErr := scanRating(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		result = append(result, rating)
		ids = append(ids, rating.ID)
	}
	if err = rows.Err(); err != nil {
		return nil, repositoryError(err)
	}
	if len(ids) == 0 {
		return result, nil
	}
	replies, err := p.db.QueryContext(ctx, `SELECT `+ratingColumns+` FROM market_ratings
 WHERE plugin_id=$1 AND parent_id = ANY($2) AND NOT hidden AND NOT deleted
 ORDER BY created_at,id`, id, pq.Array(ids))
	if err != nil {
		return nil, repositoryError(err)
	}
	defer replies.Close()
	byParent := make(map[int64]int, len(result))
	for index := range result {
		byParent[result[index].ID] = index
	}
	for replies.Next() {
		reply, scanErr := scanRating(replies)
		if scanErr != nil {
			return nil, scanErr
		}
		if reply.ParentID == nil {
			continue
		}
		if index, ok := byParent[*reply.ParentID]; ok {
			result[index].Replies = append(result[index].Replies, reply)
		}
	}
	if err = replies.Err(); err != nil {
		return nil, repositoryError(err)
	}
	return result, nil
}

// PutRating is the legacy one-review endpoint. New clients use CreateRating,
// which permits multiple reviews per user.
func (p *Postgres) PutRating(ctx context.Context, a Account, r Rating) (Rating, error) {
	return p.CreateRating(ctx, a, r)
}

// CreateRating appends one top-level review. One account may create many.
func (p *Postgres) CreateRating(ctx context.Context, a Account, r Rating) (Rating, error) {
	if a.ID == "" {
		return Rating{}, ErrUnauthorized
	}
	if err := validateRating(r); err != nil {
		return Rating{}, err
	}
	tx, err := p.db.BeginTx(ctx, nil)
	if err != nil {
		return Rating{}, repositoryError(err)
	}
	defer rollback(tx)
	owner, err := requirePublished(ctx, tx, r.PluginID)
	if err != nil {
		return Rating{}, err
	}
	if owner == a.ID {
		return Rating{}, ErrForbidden
	}
	version, err := ratingVersion(ctx, tx, r.PluginID, r.Version)
	if err != nil {
		return Rating{}, err
	}
	r.UserID = a.ID
	r.ParentID = nil
	r.Version = version
	err = tx.QueryRowContext(ctx, `INSERT INTO market_ratings(user_id,plugin_id,stars,body,version,hidden)
 SELECT $1,$2,$3,$4,$5,EXISTS(SELECT 1 FROM market_ratings
  WHERE user_id=$1 AND plugin_id=$2 AND hidden)
 RETURNING id,created_at,updated_at`,
		r.UserID, r.PluginID, r.Stars, r.Body, r.Version).Scan(&r.ID, &r.CreatedAt, &r.UpdatedAt)
	if err != nil {
		return Rating{}, repositoryError(err)
	}
	if err = tx.Commit(); err != nil {
		return Rating{}, repositoryError(err)
	}
	return r, nil
}

// UpdateRating edits one top-level review owned by the caller.
func (p *Postgres) UpdateRating(ctx context.Context, a Account, ratingID int64, r Rating) (Rating, error) {
	if a.ID == "" {
		return Rating{}, ErrUnauthorized
	}
	if err := validateRating(r); err != nil {
		return Rating{}, err
	}
	tx, err := p.db.BeginTx(ctx, nil)
	if err != nil {
		return Rating{}, repositoryError(err)
	}
	defer rollback(tx)
	owner, err := requirePublished(ctx, tx, r.PluginID)
	if err != nil {
		return Rating{}, err
	}
	if owner == a.ID {
		return Rating{}, ErrForbidden
	}
	var userID string
	var parent sql.NullInt64
	if err = tx.QueryRowContext(ctx, `SELECT user_id,parent_id FROM market_ratings
 WHERE id=$1 AND plugin_id=$2 AND NOT deleted FOR UPDATE`, ratingID, r.PluginID).Scan(&userID, &parent); err != nil {
		return Rating{}, repositoryError(err)
	}
	if parent.Valid {
		return Rating{}, ErrNotFound
	}
	if userID != a.ID {
		return Rating{}, ErrForbidden
	}
	version, err := ratingVersion(ctx, tx, r.PluginID, r.Version)
	if err != nil {
		return Rating{}, err
	}
	r.ID = ratingID
	r.UserID = a.ID
	r.ParentID = nil
	r.Version = version
	if err = tx.QueryRowContext(ctx, `UPDATE market_ratings SET stars=$1,body=$2,version=$3,updated_at=now()
 WHERE id=$4 RETURNING created_at,updated_at`, r.Stars, r.Body, r.Version, ratingID).Scan(&r.CreatedAt, &r.UpdatedAt); err != nil {
		return Rating{}, repositoryError(err)
	}
	if err = tx.Commit(); err != nil {
		return Rating{}, repositoryError(err)
	}
	return r, nil
}

// DeleteRating is the legacy plugin-scoped delete: it removes all of the
// caller's top-level reviews for the plugin.
func (p *Postgres) DeleteRating(ctx context.Context, a Account, id string) error {
	if a.ID == "" {
		return ErrUnauthorized
	}
	tx, err := p.db.BeginTx(ctx, nil)
	if err != nil {
		return repositoryError(err)
	}
	defer rollback(tx)
	if _, err = requirePublished(ctx, tx, id); err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, `UPDATE market_ratings SET deleted=true,body='',version='',updated_at=now()
 WHERE user_id=$1 AND plugin_id=$2 AND parent_id IS NULL`, a.ID, id); err != nil {
		return repositoryError(err)
	}
	return repositoryError(tx.Commit())
}

// DeleteRatingByID removes one review owned by the caller; replies are hidden
// with their parent and are never counted separately.
func (p *Postgres) DeleteRatingByID(ctx context.Context, a Account, ratingID int64) error {
	if a.ID == "" {
		return ErrUnauthorized
	}
	tx, err := p.db.BeginTx(ctx, nil)
	if err != nil {
		return repositoryError(err)
	}
	defer rollback(tx)
	var pluginID, userID string
	var parent sql.NullInt64
	if err = tx.QueryRowContext(ctx, `SELECT plugin_id,user_id,parent_id FROM market_ratings
 WHERE id=$1 AND NOT deleted FOR UPDATE`, ratingID).Scan(&pluginID, &userID, &parent); err != nil {
		return repositoryError(err)
	}
	if parent.Valid {
		return ErrNotFound
	}
	if userID != a.ID {
		return ErrForbidden
	}
	if _, err = requirePublished(ctx, tx, pluginID); err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, `UPDATE market_ratings SET deleted=true,body='',version='',updated_at=now()
 WHERE id=$1`, ratingID); err != nil {
		return repositoryError(err)
	}
	return repositoryError(tx.Commit())
}

// AddReply appends a reply to a visible top-level review.
func (p *Postgres) AddReply(ctx context.Context, a Account, pluginID string, parentID int64, body string) (Rating, error) {
	if a.ID == "" {
		return Rating{}, ErrUnauthorized
	}
	body = strings.TrimSpace(body)
	if !boundedText(body, 4000, true) {
		return Rating{}, ErrInvalid
	}
	tx, err := p.db.BeginTx(ctx, nil)
	if err != nil {
		return Rating{}, repositoryError(err)
	}
	defer rollback(tx)
	if _, err = requirePublished(ctx, tx, pluginID); err != nil {
		return Rating{}, err
	}
	var exists bool
	if err = tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM market_ratings
 WHERE id=$1 AND plugin_id=$2 AND parent_id IS NULL AND NOT hidden AND NOT deleted)`, parentID, pluginID).Scan(&exists); err != nil {
		return Rating{}, repositoryError(err)
	}
	if !exists {
		return Rating{}, ErrNotFound
	}
	parent := parentID
	r := Rating{UserID: a.ID, PluginID: pluginID, ParentID: &parent, Stars: 0, Body: body}
	if err = tx.QueryRowContext(ctx, `INSERT INTO market_ratings(user_id,plugin_id,parent_id,stars,body,version,hidden)
 SELECT $1,$2,$3,0,$4,'',EXISTS(SELECT 1 FROM market_ratings
  WHERE user_id=$1 AND plugin_id=$2 AND hidden)
 RETURNING id,created_at,updated_at`,
		r.UserID, r.PluginID, r.ParentID, r.Body).Scan(&r.ID, &r.CreatedAt, &r.UpdatedAt); err != nil {
		return Rating{}, repositoryError(err)
	}
	if err = tx.Commit(); err != nil {
		return Rating{}, repositoryError(err)
	}
	return r, nil
}

// DeleteReply removes one reply owned by the caller under the given parent.
func (p *Postgres) DeleteReply(ctx context.Context, a Account, pluginID string, parentID, replyID int64) error {
	if a.ID == "" {
		return ErrUnauthorized
	}
	tx, err := p.db.BeginTx(ctx, nil)
	if err != nil {
		return repositoryError(err)
	}
	defer rollback(tx)
	if _, err = requirePublished(ctx, tx, pluginID); err != nil {
		return err
	}
	var userID string
	var parent sql.NullInt64
	if err = tx.QueryRowContext(ctx, `SELECT user_id,parent_id FROM market_ratings
 WHERE id=$1 AND plugin_id=$2 AND NOT deleted FOR UPDATE`, replyID, pluginID).Scan(&userID, &parent); err != nil {
		return repositoryError(err)
	}
	if !parent.Valid || parent.Int64 != parentID {
		return ErrNotFound
	}
	if userID != a.ID {
		return ErrForbidden
	}
	if _, err = tx.ExecContext(ctx, `UPDATE market_ratings SET deleted=true,body='',updated_at=now() WHERE id=$1`, replyID); err != nil {
		return repositoryError(err)
	}
	return repositoryError(tx.Commit())
}

func ratingVersion(ctx context.Context, tx *sql.Tx, pluginID, requested string) (string, error) {
	var version string
	err := tx.QueryRowContext(ctx, `SELECT version FROM market_versions
 WHERE plugin_id=$1 AND state='published' AND ($2='' OR version=$2)
 ORDER BY updated_at DESC LIMIT 1`, pluginID, requested).Scan(&version)
	return version, repositoryError(err)
}

func validateRating(r Rating) error {
	if r.Stars < 1 || r.Stars > 5 || !boundedText(r.Body, 4000, false) || (r.Version != "" && !validVersion(r.Version)) {
		return ErrInvalid
	}
	return nil
}

// HideRating changes moderation for all of one user's content on a plugin and
// records an audit event atomically.
func (p *Postgres) HideRating(ctx context.Context, a Account, id, userID, reason string) error {
	if a.Role != "admin" {
		return ErrForbidden
	}
	if !boundedText(reason, 2000, true) {
		return ErrInvalid
	}
	tx, err := p.db.BeginTx(ctx, nil)
	if err != nil {
		return repositoryError(err)
	}
	defer rollback(tx)
	if _, err = requirePublished(ctx, tx, id); err != nil {
		return err
	}
	if err = affected(tx.ExecContext(ctx, "UPDATE market_ratings SET hidden=true WHERE plugin_id=$1 AND user_id=$2", id, userID)); err != nil {
		return err
	}
	if err = writeAudit(ctx, tx, a.ID, "rating.hide", id+"/"+userID, reason); err != nil {
		return err
	}
	return repositoryError(tx.Commit())
}

// Report stores an authenticated, bounded report against a published plugin.
func (p *Postgres) Report(ctx context.Context, a Account, r Report) error {
	if a.ID == "" {
		return ErrUnauthorized
	}
	if !boundedText(r.Reason, 4000, true) {
		return ErrInvalid
	}
	tx, err := p.db.BeginTx(ctx, nil)
	if err != nil {
		return repositoryError(err)
	}
	defer rollback(tx)
	if _, err = requirePublished(ctx, tx, r.PluginID); err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, "INSERT INTO market_reports(actor_id,plugin_id,reason) VALUES($1,$2,$3)", a.ID, r.PluginID, r.Reason); err != nil {
		return repositoryError(err)
	}
	return repositoryError(tx.Commit())
}
