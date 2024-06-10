package interp

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"path"
	"strconv"
	"strings"
)

// for learning yaegi

type fielder interface {
	fields(base string) map[string]any
	navigate(tokens []string) fielder
}

func (n *node) fields(base string) map[string]any {
	return map[string]any{
		"action": n.action,
		"anc":    link(base, "anc", n.anc),
		"child":  nodeLinks(base, "child", n.child),
		"debug":  n.debug,
		"findex": n.findex,
		"fnext":  link(base, "fnext", n.fnext),
		//"gen":    n.gen,
		"indent":     n.ident,
		"index":      n.index,
		"kind":       fmt.Sprintf("%d(%s)", n.kind, n.kind.String()),
		"level":      n.level,
		"meta":       n.meta,
		"nleft":      n.nleft,
		"nright":     n.nright,
		"param":      n.param,
		"pos":        n.pos,
		"redeclared": n.redeclared,
		"rval":       n.rval,
		"scope":      link(base, "scope", n.scope),
		"recv":       n.recv,
		"start":      link(base, "start", n.start),
		"sym":        n.sym,
		"tnext":      link(base, "tnext", n.tnext),
		"type":       n.typ,
		"types":      n.types,
		"val":        n.val,
	}
}

func (s *scope) fields(base string) map[string]any {
	return map[string]any{
		"anc":         link(base, "anc", s.anc),
		"child":       scopeLinks(base, "child", s.child),
		"def":         link(base, "def", s.def),
		"global":      s.global,
		"iota":        s.iota,
		"level":       s.level,
		"loop":        link(base, "loop", s.loop),
		"loopRestart": link(base, "loopRestart", s.loopRestart),
		"pkgID":       s.pkgID,
		"pkgName":     s.pkgName,
		"sym":         s.sym,
		"types":       s.types,
	}
}

func (s *scope) navigate(tokens []string) fielder {
	if len(tokens) == 0 {
		return s
	}
	switch tokens[0] {
	case "":
		return s.navigate(tokens[1:])
	case "anc":
		return s.anc.navigate(tokens[1:])
	case "def":
		return s.def.navigate(tokens[1:])
	case "loop":
		return s.loop.navigate(tokens[1:])
	case "loopRestart":
		return s.loopRestart.navigate(tokens[1:])
	default:
		return s
	}
}

func (n *node) navigate(tokens []string) fielder {
	if len(tokens) == 0 {
		return n
	}
	switch tokens[0] {
	case "":
		return n.navigate(tokens[1:])
	case "anc":
		return n.anc.navigate(tokens[1:])
	case "start":
		return n.start.navigate(tokens[1:])
	case "tnext":
		return n.tnext.navigate(tokens[1:])
	case "scope":
		return n.scope.navigate(tokens[1:])
	case "child":
		index, _ := strconv.Atoi(tokens[1])
		return n.child[index].navigate(tokens[2:])
	case "fnext":
		return n.fnext.navigate(tokens[1:])
	default:
		return n
	}
}

func link(base, field string, value any) any {
	if value == nil {
		return nil
	}
	return path.Join(base, field)
}

func nodeLinks(base, field string, nodes []*node) map[string]any {
	links := map[string]any{}
	for i, each := range nodes {
		links[strconv.FormatInt(each.index, 10)] = fmt.Sprintf("%s/%d", path.Join(base, field), i)
	}
	return links
}

func scopeLinks(base, field string, scopes []*scope) map[string]any {
	links := map[string]any{}
	for i := range scopes {
		links[strconv.Itoa(i)] = fmt.Sprintf("%s/%d", path.Join(base, field), i)
	}
	return links
}

type inspect_server struct {
	root *node
}

func (s *inspect_server) start() {
	slog.Debug("starting inspector at localhost:5656")
	http.DefaultServeMux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("content-type", "application/json")
		target := s.root.navigate(strings.Split(r.URL.Path, "/"))
		if target == nil {
			http.Error(w, "nil node", 400)
			return
		}
		if err := json.NewEncoder(w).Encode(target.fields(r.URL.Path)); err != nil {
			http.Error(w, err.Error(), 500)
		}
	})
	http.ListenAndServe(":5656", nil)
}
