package server

import (
	"net/http"
	"net/url"
	"strconv"
)

// discoverResponse is the JSON shape for GET /api/discover.
type discoverResponse struct {
	Source      string `json:"source"`
	Path        string `json:"path,omitempty"`
	Host        string `json:"host,omitempty"`
	Port        int    `json:"port,omitempty"`
	Database    string `json:"database,omitempty"`
	User        string `json:"user,omitempty"`
	Readonly    bool   `json:"readonly"`
	ProjectRoot string `json:"project_root,omitempty"`
}

// handleDiscover handles GET /api/discover.
func (s *Server) handleDiscover(w http.ResponseWriter, r *http.Request) {
	resp := discoverResponse{
		Source:      string(s.cfg.Source.Kind),
		Path:        s.cfg.Source.Path,
		Readonly:    s.cfg.Readonly,
		ProjectRoot: s.cfg.Source.ProjectRoot,
	}

	if s.cfg.Pool != nil {
		cc := s.cfg.Pool.Config().ConnConfig
		resp.Host = cc.Host
		resp.Port = int(cc.Port)
		resp.Database = cc.Database
		resp.User = cc.User
	} else if s.cfg.Source.URL != "" {
		parseURLIntoResp(s.cfg.Source.URL, &resp)
	}

	writeJSON(w, http.StatusOK, resp)
}

// parseURLIntoResp extracts host/port/database/user from a Postgres URL.
// Used only when Pool is nil.
func parseURLIntoResp(rawURL string, resp *discoverResponse) {
	u, err := url.Parse(rawURL)
	if err != nil {
		return
	}
	resp.Host = u.Hostname()
	if portStr := u.Port(); portStr != "" {
		if p, err := strconv.Atoi(portStr); err == nil {
			resp.Port = p
		}
	}
	if u.Path != "" {
		resp.Database = u.Path[1:] // strip leading "/"
	}
	if u.User != nil {
		resp.User = u.User.Username()
	}
}
