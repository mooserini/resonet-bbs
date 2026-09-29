package main

import (
	"os"
	"strings"
)

// upstreamSiteName is the software's own name. Public-facing text uses the
// fork's configured name (identity.env -> WOLFBBS_BBS_NAME, or site.name in
// /admin/config) and only falls back to this when nothing is configured.
const upstreamSiteName = "WolfBBS"

func defaultSiteName() string {
	if name := strings.TrimSpace(os.Getenv("WOLFBBS_BBS_NAME")); name != "" {
		return name
	}
	return upstreamSiteName
}
