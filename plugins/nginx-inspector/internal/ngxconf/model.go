package ngxconf

import "strings"

// Walk visits every directive depth-first, including block bodies.
func Walk(directives []Directive, visit func(d *Directive) bool) {
	for i := range directives {
		d := &directives[i]
		if !visit(d) {
			continue
		}
		if len(d.Block) > 0 {
			Walk(d.Block, visit)
		}
	}
}

// FindByName returns all directives with the given name, anywhere in the tree.
func FindByName(directives []Directive, name string) []*Directive {
	var out []*Directive
	Walk(directives, func(d *Directive) bool {
		if d.Name == name {
			out = append(out, d)
		}
		return true
	})
	return out
}

// FindServers returns server blocks (http and stream contexts).
func FindServers(directives []Directive) []*Directive {
	var out []*Directive
	Walk(directives, func(d *Directive) bool {
		// Do not descend into nested location blocks: a "server" directive
		// there is not an http server.
		if d.Name == "location" {
			return false
		}
		if d.Name == "server" {
			out = append(out, d)
		}
		return true
	})
	return out
}

// FindUpstreams returns upstream blocks.
func FindUpstreams(directives []Directive) []*Directive {
	var out []*Directive
	Walk(directives, func(d *Directive) bool {
		if d.Name == "location" {
			return false
		}
		if d.Name == "upstream" {
			out = append(out, d)
		}
		return true
	})
	return out
}

// FindLocations returns all location blocks inside one server, including
// nested ones.
func FindLocations(server *Directive) []*Directive {
	var out []*Directive
	var walk func(dirs []Directive)
	walk = func(dirs []Directive) {
		for i := range dirs {
			d := &dirs[i]
			if d.Name == "location" {
				out = append(out, d)
			}
			if len(d.Block) > 0 && d.Name != "location" {
				walk(d.Block)
			} else if d.Name == "location" && len(d.Block) > 0 {
				walk(d.Block)
			}
		}
	}
	walk(server.Block)
	return out
}

// ServerName returns the server_name values of one server block.
func ServerName(server *Directive) []string {
	var names []string
	for _, d := range server.Block {
		if d.Name == "server_name" {
			names = append(names, d.ArgValues()...)
		}
	}
	return names
}

// ListenAddresses returns the listen values of one server block.
func ListenAddresses(server *Directive) []string {
	var addrs []string
	for _, d := range server.Block {
		if d.Name == "listen" {
			addrs = append(addrs, d.FirstArg())
		}
	}
	return addrs
}

// FirstDirective returns the first argument of the named child directive.
func FirstDirective(directives []Directive, name string) string {
	for i := range directives {
		if directives[i].Name == name {
			return directives[i].FirstArg()
		}
	}
	return ""
}

// AllDirectiveArgs returns the first argument of every occurrence of name.
func AllDirectiveArgs(directives []Directive, name string) []string {
	var out []string
	for i := range directives {
		if directives[i].Name == name {
			out = append(out, directives[i].FirstArg())
		}
	}
	return out
}

// ServerEnabled reports whether the server listens on TLS based on listen
// parameters or ssl_certificate presence.
func ServerEnabled(server *Directive) bool {
	for _, d := range server.Block {
		if d.Name == "listen" {
			for _, a := range d.Args {
				if a.Value == "ssl" {
					return true
				}
			}
		}
	}
	return len(FindByName(server.Block, "ssl_certificate")) > 0
}

// UpstreamServer is one member entry of an upstream block.
type UpstreamServer struct {
	Address string   `json:"address"`
	Params  []string `json:"params,omitempty"`
	Line    int      `json:"line"`
}

// UpstreamMembers parses the server entries of an upstream block.
func UpstreamMembers(upstream *Directive) []UpstreamServer {
	var out []UpstreamServer
	for i := range upstream.Block {
		d := &upstream.Block[i]
		if d.Name == "server" && d.FirstArg() != "" {
			out = append(out, UpstreamServer{Address: d.FirstArg(), Params: d.ArgValues()[1:], Line: d.Line})
		}
	}
	return out
}

// ProxyPass returns the proxy_pass targets of one location block.
func ProxyPass(location *Directive) string {
	return FirstDirective(location.Block, "proxy_pass")
}

// LocationPath returns the location's match pattern (first argument without
// modifiers like "=" or "^~").
func LocationPath(location *Directive) string {
	for _, a := range location.Args {
		if strings.HasPrefix(a.Value, "/") || strings.HasPrefix(a.Value, "~") || a.Value == "=" || a.Value == "^~" {
			return a.Value
		}
	}
	if len(location.Args) > 0 {
		return location.Args[len(location.Args)-1].Value
	}
	return ""
}

// IsInclude reports whether the directive is an include and returns its patterns.
func IsInclude(d *Directive) ([]string, bool) {
	if d.Name != "include" || len(d.Args) == 0 {
		return nil, false
	}
	return d.ArgValues(), true
}
