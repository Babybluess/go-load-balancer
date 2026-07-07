package proxy

import (
	"sort"
	"strings"
)

// Router dispatches a request path to the Balancer registered for the
// longest matching path prefix
type Router struct {
	routes   map[string]*Balancer
	prefixes []string
}

func NewRouter(routes map[string]*Balancer) *Router {
	prefixes := make([]string, 0, len(routes))
	for prefix := range routes {
		prefixes = append(prefixes, prefix)
	}
	sort.Slice(prefixes, func(i, j int) bool {
		return len(prefixes[i]) > len(prefixes[j])
	})

	return &Router{routes: routes, prefixes: prefixes}
}

// Match returns the Balancer registered for the longest prefix matching
// path, or nil if no registered prefix matches.
func (rt *Router) Match(path string) *Balancer {
	for _, prefix := range rt.prefixes {
		if strings.HasPrefix(path, prefix) {
			return rt.routes[prefix]
		}
	}
	return nil
}

// Routes returns the prefix -> Balancer mapping
func (rt *Router) Routes() map[string]*Balancer {
	return rt.routes
}
