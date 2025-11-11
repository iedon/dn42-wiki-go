package site

func (s *Service) routeIsPrivateFromRel(rel string) bool {
	route := routeFromPath(rel, s.homeDoc)
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
	info, ok := s.analyzeRequestPath(requestPath)
	if !ok {
		return "", ErrInvalidPath
	}
	if info.relative == directoryPageRoute {
		return directoryPageRoute, nil
	}
	if info.relative == "/" {
		return "/", nil
	}
	rel, err := normalizeRelPath(info.candidate, s.homeDoc)
	if err != nil {
		return "", err
	}
	return routeFromPath(rel, s.homeDoc), nil
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
