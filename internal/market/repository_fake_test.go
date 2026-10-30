package market

import (
	"context"
	"encoding/base64"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/Syngnat/GoNavi-Plugin-Market/internal/marketfilter"
	"golang.org/x/mod/semver"
)

type testRelease struct {
	submission  Submission
	packageData ValidatedPackage
	owner       string
	sourceURL   string
}
type testSession struct {
	accountID string
	expires   time.Time
}

type memoryRepo struct {
	mu           sync.Mutex
	accounts     map[string]Account
	sessions     map[string]testSession
	flows        map[string]AuthFlow
	releases     map[string]testRelease
	ratings      map[int64]Rating
	hidden       map[int64]bool
	moderated    map[string]bool
	nextRatingID int64
	downloads    map[string]int64
	audit        []Audit
}

var _ Repository = (*memoryRepo)(nil)

func newMemoryRepo() *memoryRepo {
	return &memoryRepo{accounts: map[string]Account{}, sessions: map[string]testSession{}, flows: map[string]AuthFlow{},
		releases: map[string]testRelease{}, ratings: map[int64]Rating{}, hidden: map[int64]bool{},
		moderated: map[string]bool{}, downloads: map[string]int64{}}
}

func (m *memoryRepo) published(id string) bool {
	for _, v := range m.releases {
		if v.submission.PluginID == id && v.submission.State == "published" {
			return true
		}
	}
	return false
}

func (m *memoryRepo) plugin(v testRelease) MarketPlugin {
	pkg := v.packageData
	p := MarketPlugin{ID: pkg.ID, Name: pkg.Name, Description: pkg.Description, Author: pkg.Author, Version: pkg.Version,
		PublisherID: m.accounts[v.owner].PublisherID, Official: m.accounts[v.owner].Official, MinHostVersion: pkg.MinHostVersion, Platforms: pkg.Platforms,
		SourceURL: v.sourceURL, Category: categoryFromManifest(pkg.Manifest),
		Manifest: append([]byte(nil), pkg.Manifest...), UpdatedAt: v.submission.UpdatedAt, DownloadCount: m.downloads[pkg.ID],
		Artifact: &Artifact{SHA256: pkg.SHA256, SizeBytes: pkg.SizeBytes, Platform: "any", Version: pkg.Version}}
	for id, r := range m.ratings {
		if r.PluginID == pkg.ID && r.ParentID == nil && !m.hidden[id] {
			p.RatingCount++
			p.RatingAverage += float64(r.Stars)
		}
	}
	if p.RatingCount > 0 {
		p.RatingAverage /= float64(p.RatingCount)
	}
	return p
}

func (m *memoryRepo) Catalog(ctx context.Context, q CatalogQuery) (CatalogPage, error) {
	if err := validateCatalogQuery(q); err != nil {
		return CatalogPage{}, err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	latest := map[string]MarketPlugin{}
	for _, v := range m.releases {
		if v.submission.State != "published" {
			continue
		}
		p := m.plugin(v)
		if q.HostVersion != "" && p.MinHostVersion != "" && semver.Compare("v"+p.MinHostVersion, "v"+q.HostVersion) > 0 {
			continue
		}
		compatible := q.Platform == ""
		for _, platform := range p.Platforms {
			if platform == "any" || platform == q.Platform {
				compatible = true
			}
		}
		if !compatible || !marketfilter.MatchesPlatforms(marketfilter.PlatformValues(q.Filters), p.Platforms) {
			continue
		}
		if old, ok := latest[p.ID]; !ok || old.UpdatedAt.Before(p.UpdatedAt) {
			latest[p.ID] = p
		}
	}
	items := make([]MarketPlugin, 0, len(latest))
	for _, p := range latest {
		if q.Category != "" && p.Category != q.Category {
			continue
		}
		if q.Query != "" && !strings.Contains(strings.ToLower(p.ID+" "+p.Name+" "+p.Description), strings.ToLower(q.Query)) {
			continue
		}
		if !marketfilter.Matches(q.Filters, p.Category, p.Author, p.PublisherID, p.Official, p.Platforms) {
			continue
		}
		items = append(items, p)
	}
	mean, total := 3.0, 0
	sum := 0.0
	for id, r := range m.ratings {
		if r.ParentID == nil && !m.hidden[id] && m.published(r.PluginID) {
			sum += float64(r.Stars)
			total++
		}
	}
	if total > 0 {
		mean = sum / float64(total)
	}
	sort.Slice(items, func(i, j int) bool {
		a, b := items[i], items[j]
		if a.Official != b.Official {
			return a.Official
		}
		if q.Sort == "downloads" && a.DownloadCount != b.DownloadCount {
			return a.DownloadCount > b.DownloadCount
		}
		if q.Sort == "rating" {
			scoreA := (a.RatingAverage*float64(a.RatingCount) + 10*mean) / (float64(a.RatingCount) + 10)
			scoreB := (b.RatingAverage*float64(b.RatingCount) + 10*mean) / (float64(b.RatingCount) + 10)
			if scoreA != scoreB {
				return scoreA > scoreB
			}
		}
		if !a.UpdatedAt.Equal(b.UpdatedAt) {
			return a.UpdatedAt.After(b.UpdatedAt)
		}
		return a.ID < b.ID
	})
	page := CatalogPage{Items: []MarketPlugin{}}
	if q.Offset >= len(items) {
		return page, nil
	}
	end := q.Offset + q.Limit
	if end < len(items) {
		if end <= 10000 {
			page.NextCursor = base64.RawURLEncoding.EncodeToString([]byte(strconv.Itoa(end)))
		}
	} else {
		end = len(items)
	}
	page.Items = items[q.Offset:end]
	return page, nil
}

func (m *memoryRepo) Plugin(ctx context.Context, id string) (MarketPlugin, error) {
	versions, err := m.Versions(ctx, id)
	if err != nil {
		return MarketPlugin{}, err
	}
	return versions[0], nil
}

func (m *memoryRepo) Versions(ctx context.Context, id string) ([]MarketPlugin, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	items := []MarketPlugin{}
	for _, v := range m.releases {
		if v.submission.PluginID == id && v.submission.State == "published" {
			items = append(items, m.plugin(v))
		}
	}
	if len(items) == 0 {
		return nil, ErrNotFound
	}
	sort.Slice(items, func(i, j int) bool { return items[i].UpdatedAt.After(items[j].UpdatedAt) })
	return items, nil
}

func (m *memoryRepo) Artifact(ctx context.Context, id, version, platform string) (Artifact, error) {
	versions, err := m.Versions(ctx, id)
	if err != nil {
		return Artifact{}, err
	}
	for _, v := range versions {
		if v.Version == version {
			for _, supported := range v.Platforms {
				if platform == "" || supported == "any" || supported == platform {
					return *v.Artifact, nil
				}
			}
		}
	}
	return Artifact{}, ErrNotFound
}

func (m *memoryRepo) PublishedArtifact(ctx context.Context, digest string) (Artifact, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, v := range m.releases {
		if v.submission.State == "published" && v.packageData.SHA256 == digest {
			return *m.plugin(v).Artifact, nil
		}
	}
	return Artifact{}, ErrNotFound
}

func (m *memoryRepo) CountDownload(ctx context.Context, digest string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, v := range m.releases {
		if v.submission.State == "published" && v.packageData.SHA256 == digest {
			m.downloads[v.submission.PluginID]++
			return nil
		}
	}
	return ErrNotFound
}

func (m *memoryRepo) Ratings(ctx context.Context, id string) ([]Rating, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if !m.published(id) {
		return nil, ErrNotFound
	}
	result := []Rating{}
	for ratingID, r := range m.ratings {
		if r.PluginID == id && r.ParentID == nil && !m.hidden[ratingID] {
			result = append(result, r)
		}
	}
	sort.Slice(result, func(i, j int) bool {
		if !result[i].UpdatedAt.Equal(result[j].UpdatedAt) {
			return result[i].UpdatedAt.After(result[j].UpdatedAt)
		}
		return result[i].ID > result[j].ID
	})
	for index := range result {
		for replyID, reply := range m.ratings {
			if reply.ParentID != nil && *reply.ParentID == result[index].ID && !m.hidden[replyID] {
				result[index].Replies = append(result[index].Replies, reply)
			}
		}
		sort.Slice(result[index].Replies, func(i, j int) bool {
			return result[index].Replies[i].CreatedAt.Before(result[index].Replies[j].CreatedAt)
		})
	}
	return result, nil
}

func (m *memoryRepo) PutRating(ctx context.Context, a Account, r Rating) (Rating, error) {
	return m.CreateRating(ctx, a, r)
}

func (m *memoryRepo) CreateRating(ctx context.Context, a Account, r Rating) (Rating, error) {
	if a.ID == "" {
		return Rating{}, ErrUnauthorized
	}
	if err := validateRating(r); err != nil {
		return Rating{}, err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	found := false
	for _, v := range m.releases {
		if v.submission.PluginID == r.PluginID && v.submission.State == "published" {
			if v.owner == a.ID {
				return Rating{}, ErrForbidden
			}
			if r.Version == "" || v.submission.Version == r.Version {
				r.Version = v.submission.Version
				found = true
			}
		}
	}
	if !found {
		return Rating{}, ErrNotFound
	}
	m.nextRatingID++
	r.ID = m.nextRatingID
	r.UserID = a.ID
	r.ParentID = nil
	r.CreatedAt = time.Now().UTC()
	r.UpdatedAt = r.CreatedAt
	m.ratings[r.ID] = r
	if m.moderated[a.ID+"/"+r.PluginID] {
		m.hidden[r.ID] = true
	}
	return r, nil
}

func (m *memoryRepo) UpdateRating(ctx context.Context, a Account, ratingID int64, r Rating) (Rating, error) {
	if err := validateRating(r); err != nil {
		return Rating{}, err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	current, ok := m.ratings[ratingID]
	if !ok || current.ParentID != nil {
		return Rating{}, ErrNotFound
	}
	if current.UserID != a.ID {
		return Rating{}, ErrForbidden
	}
	for _, v := range m.releases {
		if v.submission.PluginID == current.PluginID && v.submission.State == "published" {
			if v.owner == a.ID {
				return Rating{}, ErrForbidden
			}
			if r.Version == "" || v.submission.Version == r.Version {
				r.Version = v.submission.Version
			}
		}
	}
	current.Stars = r.Stars
	current.Body = r.Body
	current.Version = r.Version
	current.UpdatedAt = time.Now().UTC()
	m.ratings[ratingID] = current
	return current, nil
}

func (m *memoryRepo) DeleteRating(ctx context.Context, a Account, id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if !m.published(id) {
		return ErrNotFound
	}
	for ratingID, r := range m.ratings {
		if r.PluginID == id && r.UserID == a.ID {
			delete(m.ratings, ratingID)
		}
	}
	return nil
}

func (m *memoryRepo) DeleteRatingByID(ctx context.Context, a Account, ratingID int64) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	r, ok := m.ratings[ratingID]
	if !ok || r.ParentID != nil {
		return ErrNotFound
	}
	if r.UserID != a.ID {
		return ErrForbidden
	}
	delete(m.ratings, ratingID)
	for replyID, reply := range m.ratings {
		if reply.ParentID != nil && *reply.ParentID == ratingID {
			delete(m.ratings, replyID)
		}
	}
	return nil
}

func (m *memoryRepo) AddReply(ctx context.Context, a Account, pluginID string, parentID int64, body string) (Rating, error) {
	body = strings.TrimSpace(body)
	if !boundedText(body, 4000, true) {
		return Rating{}, ErrInvalid
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	parent, ok := m.ratings[parentID]
	if !ok || parent.PluginID != pluginID || parent.ParentID != nil || m.hidden[parentID] {
		return Rating{}, ErrNotFound
	}
	m.nextRatingID++
	parentCopy := parentID
	now := time.Now().UTC()
	r := Rating{ID: m.nextRatingID, UserID: a.ID, PluginID: pluginID, ParentID: &parentCopy,
		Stars: 0, Body: body, CreatedAt: now, UpdatedAt: now}
	m.ratings[r.ID] = r
	if m.moderated[a.ID+"/"+pluginID] {
		m.hidden[r.ID] = true
	}
	return r, nil
}

func (m *memoryRepo) DeleteReply(ctx context.Context, a Account, pluginID string, parentID, replyID int64) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	r, ok := m.ratings[replyID]
	if !ok || r.PluginID != pluginID || r.ParentID == nil || *r.ParentID != parentID {
		return ErrNotFound
	}
	if r.UserID != a.ID {
		return ErrForbidden
	}
	delete(m.ratings, replyID)
	return nil
}

func (m *memoryRepo) HideRating(ctx context.Context, a Account, id, user, reason string) error {
	if a.Role != "admin" {
		return ErrForbidden
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	found := false
	for ratingID, r := range m.ratings {
		if r.PluginID == id && r.UserID == user {
			m.hidden[ratingID] = true
			found = true
		}
	}
	if !found {
		return ErrNotFound
	}
	m.moderated[user+"/"+id] = true
	return nil
}

func (m *memoryRepo) Report(ctx context.Context, a Account, r Report) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if !m.published(r.PluginID) {
		return ErrNotFound
	}
	if strings.TrimSpace(r.Reason) == "" || len(r.Reason) > 4000 {
		return ErrInvalid
	}
	return nil
}
