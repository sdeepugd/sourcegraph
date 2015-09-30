package app

import (
	"net/http"
	"net/url"
	"os"

	"sourcegraph.com/sourcegraph/go-sourcegraph/sourcegraph"
	"sourcegraph.com/sqs/pbtypes"
	"src.sourcegraph.com/sourcegraph/app/internal/schemautil"
	"src.sourcegraph.com/sourcegraph/app/internal/tmpl"
	"src.sourcegraph.com/sourcegraph/util/handlerutil"
	"src.sourcegraph.com/sourcegraph/util/httputil/httpctx"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
)

func serveHomeDashboard(w http.ResponseWriter, r *http.Request) error {
	ctx := httpctx.FromRequest(r)
	cl := sourcegraph.NewClientFromContext(ctx)

	var listOpts sourcegraph.ListOptions
	if err := schemautil.Decode(&listOpts, r.URL.Query()); err != nil {
		return err
	}

	if listOpts.PerPage == 0 {
		listOpts.PerPage = 50
	}

	repos, err := cl.Repos.List(ctx, &sourcegraph.RepoListOptions{ListOptions: listOpts})
	if err != nil {
		if grpc.Code(err) == codes.Unauthenticated {
			return serveWelcomeInterstitial(w, r)
		}
		return err
	}
	var template string
	var orgUsers []string
	if len(repos.Repos) > 0 {
		userPerms, err := cl.RegisteredClients.ListUserPermissions(ctx, &sourcegraph.RegisteredClientSpec{})
		if err == nil {
			for _, perms := range userPerms.UserPermissions {
				user, err := cl.Users.Get(ctx, &sourcegraph.UserSpec{UID: perms.UID})
				if err == nil {
					orgUsers = append(orgUsers, user.Login)
				}
			}
		}
		template = "home/dashboard.html"
	} else {
		template = "home/new.html"
	}

	return tmpl.Exec(r, w, template, http.StatusOK, nil, &struct {
		Repos    []*sourcegraph.Repo
		SGPath   string
		OrgUsers []string
		tmpl.Common
	}{
		Repos:    repos.Repos,
		SGPath:   os.Getenv("SGPATH"),
		OrgUsers: orgUsers,
	})
}

func serveWelcomeInterstitial(w http.ResponseWriter, r *http.Request) error {
	cl := handlerutil.APIClient(r)
	ctx := httpctx.FromRequest(r)

	conf, err := cl.Meta.Config(ctx, &pbtypes.Void{})
	if err != nil {
		return err
	}
	u, err := url.Parse(conf.FederationRootURL)
	if err != nil {
		return err
	}
	return tmpl.Exec(r, w, "home/welcome.html", http.StatusOK, nil, &struct {
		RootHostname string
		tmpl.Common
	}{
		RootHostname: u.Host,
	})
}
