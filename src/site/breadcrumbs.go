package site

import (
	"strings"

	"github.com/iedon/dn42-wiki-go/templatex"
)

func buildBreadcrumbs(route, title, base string) []templatex.Breadcrumb {
	trimmedBase := strings.Trim(strings.TrimSpace(base), "/")
	rootHref := directoryPageHref(trimmedBase)

	crumbs := make([]templatex.Breadcrumb, 0, 4)
	crumbs = append(crumbs, templatex.Breadcrumb{Title: directoryPageTitle, Path: rootHref})

	normRoute := strings.Trim(route, "/")
	if normRoute == "" {
		home := templatex.Breadcrumb{
			Title:   title,
			Current: true,
		}
		crumbs = append(crumbs, home)
		return crumbs
	}

	segments := strings.Split(normRoute, "/")
	for i, segment := range segments {
		if segment == "" {
			continue
		}
		isLast := i == len(segments)-1
		crumb := templatex.Breadcrumb{
			Title:   segment,
			Current: isLast,
		}
		if isLast {
			crumb.Title = title
			crumb.Path = ""
		} else {
			anchor := breadcrumbAnchor(segment)
			if anchor != "" {
				crumb.Path = rootHref + "#" + anchor
			} else {
				crumb.Path = rootHref
			}
		}
		crumbs = append(crumbs, crumb)
	}

	return crumbs
}
