// Copyright 2019 Drone IO, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package builds

import (
	"net/http"

	"github.com/drone/drone/core"
	"github.com/drone/drone/handler/api/render"
	"github.com/drone/drone/handler/api/request"
	"github.com/drone/go-scm/scm"

	"github.com/go-chi/chi"
)

// HandleCreate returns an http.HandlerFunc that processes http
// requests to create a build for the specified commit.
func HandleCreate(
	users core.UserStore,
	repos core.RepositoryStore,
	commits core.CommitService,
	triggerer core.Triggerer,
) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var (
			ctx          = r.Context()
			namespace    = chi.URLParam(r, "owner")
			name         = chi.URLParam(r, "name")
			sha          = r.FormValue("commit")
			before       = r.FormValue("before")
			after        = r.FormValue("after")
			branch       = r.FormValue("branch")
			sourceBranch = r.FormValue("source_branch")
			targetBranch = r.FormValue("target_branch")
			ref          = r.FormValue("ref")
			event        = r.FormValue("event")
			trigger      = r.FormValue("trigger")
			link         = r.FormValue("link")
			title        = r.FormValue("title")
			user, _      = request.UserFrom(ctx)
		)

		repo, err := repos.FindName(ctx, namespace, name)
		if err != nil {
			render.NotFound(w, err)
			return
		}

		owner, err := users.Find(ctx, repo.UserID)
		if err != nil {
			render.NotFound(w, err)
			return
		}

		// if the user does not provide a branch, assume the
		// default repository branch.
		if branch == "" {
			branch = repo.Branch
		}
		if sourceBranch == "" {
			sourceBranch = branch
		}
		if targetBranch == "" {
			targetBranch = branch
		}
		if ref == "" {
			// expand the branch to a git reference.
			ref = scm.ExpandRef(branch, "refs/heads")
		}

		var commit *core.Commit
		if sha != "" {
			commit, err = commits.Find(ctx, owner, repo.Slug, sha)
		} else {
			commit, err = commits.FindRef(ctx, owner, repo.Slug, ref)
		}
		if err != nil {
			render.NotFound(w, err)
			return
		}
		if event == "" {
			event = core.EventCustom
		}
		if trigger == "" {
			trigger = user.Login
		}
		if link == "" {
			link = commit.Link
		}
		if before == "" {
			before = commit.Sha
		}
		if after == "" {
			after = commit.Sha
		}

		hook := &core.Hook{
			Trigger:      trigger,
			Event:        event,
			Link:         link,
			Timestamp:    commit.Author.Date,
			Title:        title,
			Message:      commit.Message,
			Before:       before,
			After:        after,
			Ref:          ref,
			Source:       sourceBranch,
			Target:       targetBranch,
			Author:       commit.Author.Login,
			AuthorName:   commit.Author.Name,
			AuthorEmail:  commit.Author.Email,
			AuthorAvatar: commit.Author.Avatar,
			Sender:       trigger,
			Params:       map[string]string{},
		}

		for key, value := range r.URL.Query() {
			if key == "access_token" ||
				key == "commit" ||
				key == "branch" {
				continue
			}
			if len(value) == 0 {
				continue
			}
			hook.Params[key] = value[0]
		}

		result, err := triggerer.Trigger(r.Context(), repo, hook)
		if err != nil {
			render.InternalError(w, err)
		} else {
			render.JSON(w, result, 200)
		}
	}
}
