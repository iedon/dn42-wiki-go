package site

import (
	"context"
	"html/template"
	"os"
	"strings"
)

func (s *Service) refreshLayout(ctx context.Context) error {
	_ = ctx
	var headerHTML, footerHTML, sidebarHTML, serverFooterHTML template.HTML

	if !s.cfg.IgnoreHeader {
		if fragment, err := s.documents.RenderFragment("_Header.md"); err != nil {
			if !os.IsNotExist(err) {
				return err
			}
		} else {
			headerHTML = template.HTML(fragment.HTML)
		}
	}

	if !s.cfg.IgnoreFooter {
		if fragment, err := s.documents.RenderFragment("_Footer.md"); err != nil {
			if !os.IsNotExist(err) {
				return err
			}
		} else {
			footerHTML = template.HTML(fragment.HTML)
		}
	}

	if fragment, err := s.documents.RenderFragment("_Sidebar.md"); err != nil {
		if !os.IsNotExist(err) {
			return err
		}
	} else {
		sidebarHTML = template.HTML(fragment.HTML)
	}

	if trimmed := strings.TrimSpace(s.cfg.ServerFooter); trimmed != "" {
		rendered, err := s.renderer.Render([]byte(trimmed))
		if err != nil {
			return err
		}
		serverFooterHTML = template.HTML(rendered.HTML)
	}

	s.layout.Update(headerHTML, footerHTML, serverFooterHTML, sidebarHTML)
	return nil
}

func (s *Service) rebuildSearchIndex(ctx context.Context) error {
	files, err := s.documents.ListTracked(ctx)
	if err != nil {
		return err
	}
	docs, err := s.renderDocuments(ctx, files)
	if err != nil {
		return err
	}
	payload, err := buildSearchIndex(docs)
	if err != nil {
		return err
	}
	s.search.Update(payload)
	return nil
}
