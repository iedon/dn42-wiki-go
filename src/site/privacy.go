package site

import "strings"

func (s *Service) routeIsPrivateFromRel(rel string) bool {
	route := routeFromPath(rel)
	return s.routeIsPrivate(route)
}

func (s *Service) routeIsPrivate(route string) bool {
	if !s.cfg.Live {
		return false
	}
	return s.cfg.IsPathPrivate(route)
}

func (s *Service) ensureRouteAccessible(rel string) error {
	if s.routeIsPrivateFromRel(rel) {
		return ErrForbiddenRoute
	}
	return nil
}

func (s *Service) routeFromRequestPath(requestPath string) (string, error) {
	route := sanitizeRoute(requestPath)
	if route == directoryPageRoute {
		return directoryPageRoute, nil
	}
	if route == "/" {
		home := ensureHomeDoc(s.cfg.HomeDoc)
		return routeFromPath(home), nil
	}

	trimmed := strings.TrimPrefix(route, "/")
	trimmed = strings.TrimSuffix(trimmed, "/")
	lower := strings.ToLower(trimmed)
	if strings.HasSuffix(lower, ".html") {
		trimmed = trimmed[:len(trimmed)-len(".html")]
	}

	rel, err := normalizeRelPath(trimmed, s.cfg.HomeDoc)
	if err != nil {
		return "", err
	}
	return routeFromPath(rel), nil
}

// EnsureRequestAccessible validates whether the provided HTTP route is accessible in live mode.
func (s *Service) EnsureRequestAccessible(requestPath string) error {
	if !s.cfg.Live {
		return nil
	}
	route, err := s.routeFromRequestPath(requestPath)
	if err != nil {
		return err
	}
	if s.routeIsPrivate(route) {
		return ErrForbiddenRoute
	}
	return nil
}
