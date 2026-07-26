package tools

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"sort"

	"buatpostingan/internal/domain/service"
)

var _ service.PageWorkspaceManager = (*Registry)(nil)

func (r *Registry) ListPageWorkspaces(_ context.Context) ([]service.PageWorkspace, error) {
	if r.pages == nil || r.pages.root == "" {
		return nil, errors.New("pages unavailable")
	}
	ids, err := r.pages.pageIDs("")
	if err != nil {
		return nil, err
	}
	pages := make([]service.PageWorkspace, 0, len(ids))
	for _, id := range ids {
		page, err := r.pages.workspace(id)
		if err != nil {
			return nil, err
		}
		pages = append(pages, page)
	}
	return pages, nil
}

func (r *Registry) PublishPageWorkspace(_ context.Context, pageID string) (service.PageWorkspace, error) {
	if r.pages == nil {
		return service.PageWorkspace{}, errors.New("pages unavailable")
	}
	env := r.pages.publish(pageID)
	if !env.OK {
		return service.PageWorkspace{}, pageEnvelopeError(env)
	}
	return r.pages.workspace(pageID)
}

func (r *Registry) UnpublishPageWorkspace(_ context.Context, pageID string) (service.PageWorkspace, error) {
	if r.pages == nil {
		return service.PageWorkspace{}, errors.New("pages unavailable")
	}
	env := r.pages.unpublish(pageID)
	if !env.OK {
		return service.PageWorkspace{}, pageEnvelopeError(env)
	}
	return r.pages.workspace(pageID)
}

func (r *Registry) DeletePageWorkspace(_ context.Context, pageID string) error {
	if r.pages == nil {
		return errors.New("pages unavailable")
	}
	id, env := validatePageID("page_delete", pageID)
	if env != nil {
		return pageEnvelopeError(*env)
	}
	path := filepath.Join(r.pages.root, id)
	info, err := os.Lstat(path)
	if os.IsNotExist(err) || (err == nil && (!info.IsDir() || info.Mode()&os.ModeSymlink != 0)) {
		return errors.New("page workspace does not exist")
	}
	if err != nil {
		return err
	}
	if r.pages.isPublished(id) {
		env := r.pages.unpublish(id)
		if !env.OK {
			return pageEnvelopeError(env)
		}
	}
	return os.RemoveAll(path)
}

func (p *pagesFS) workspace(id string) (service.PageWorkspace, error) {
	if !pageIDPattern.MatchString(id) {
		return service.PageWorkspace{}, errors.New("invalid page id")
	}
	root := filepath.Join(p.root, id)
	entries := make([]service.PageEntry, 0)
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if path == root || entry.Type()&os.ModeSymlink != 0 {
			if entry.IsDir() && path != root {
				return filepath.SkipDir
			}
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		kind := "file"
		if entry.IsDir() {
			kind = "directory"
		}
		entries = append(entries, service.PageEntry{Path: filepath.ToSlash(rel), Type: kind})
		return nil
	})
	if err != nil {
		return service.PageWorkspace{}, err
	}
	sort.Slice(entries, func(i, j int) bool {
		if entries[i].Type != entries[j].Type {
			return entries[i].Type == "directory"
		}
		return entries[i].Path < entries[j].Path
	})
	return service.PageWorkspace{ID: id, Published: p.isPublished(id), Entries: entries}, nil
}

func pageEnvelopeError(env service.ToolEnvelope) error {
	if env.Error != nil {
		if message := asString(env.Error["message"]); message != "" {
			return errors.New(message)
		}
	}
	return errors.New("page action failed")
}
