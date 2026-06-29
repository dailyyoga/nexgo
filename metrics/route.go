package metrics

import "github.com/gin-gonic/gin"

// DefaultPath is the conventional mount point for the metrics endpoint.
const DefaultPath = "/metrics"

// RegisterRoute mounts reg's Prometheus handler on the gin engine at path. When
// path is empty, DefaultPath ("/metrics") is used.
//
// The handler is mounted directly on the engine, so it is not affected by
// middleware applied to specific route groups. Callers that need to place the
// endpoint behind auth can register it on an authenticated *gin.Engine or wire
// the handler manually via reg.Handler().
func RegisterRoute(r *gin.Engine, reg *Registry, path string) {
	if path == "" {
		path = DefaultPath
	}
	r.GET(path, gin.WrapH(reg.Handler()))
}
