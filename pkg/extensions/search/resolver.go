package search

//go:generate go run github.com/99designs/gqlgen

import (
	"context"
	"github.com/anuvu/zot/pkg/log"
	"github.com/anuvu/zot/pkg/storage"
) // THIS CODE IS A STARTING POINT ONLY. IT WILL NOT BE UPDATED WITH SCHEMA CHANGES.

type Resolver struct {
	ImageStore *storage.ImageStore `gqlgen:"-",json:"-"`
	Log        log.Logger
}

func (r *Resolver) Query() QueryResolver {
	return &queryResolver{r}
}

type queryResolver struct{ *Resolver }

func (r *queryResolver) Repositories(ctx context.Context, name *string) ([]*Repository, error) {
	repos, err := r.ImageStore.GetRepositories()
	if err != nil {
		return nil, err
	}

	res := make([]*Repository, 0)
	for _, repo := range repos {
		if name == nil || repo == *name {
			res = append(res, &Repository{Name: repo})
		}
	}
	return res, nil
}

func (r *queryResolver) Search(ctx context.Context, text string) ([]SearchResult, error) {
	panic("not implemented")
}
