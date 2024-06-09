package interp

import (
	"context"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
)

// this file contains extensions to the interp package
// to get varvoy working with yaegi (v0.16.1)

// CompilePackage returns a Program for running all its inits which includes main.
func (interp *Interpreter) CompilePackage(mainDir string) (*Program, error) {
	if isFile(interp.filesystem, mainDir) {
		return nil, fmt.Errorf("mainDir must be a directory:%s", mainDir)
	}
	return interp.loadSources(".", mainDir)
}

// loadSources calls gta on the source code for the package identified by
// importPath. rPath is the relative path to the directory containing the source
// code for the package.
// NOTE: this implementation is a modified version of *Interpreter.importSrc(...)
func (interp *Interpreter) loadSources(rPath, importPath string) (*Program, error) {
	var dir string = importPath
	var err error

	files, err := fs.ReadDir(interp.opt.filesystem, importPath)
	if err != nil {
		return nil, err
	}

	var initNodes []*node
	var rootNodes []*node
	revisit := make(map[string][]*node)

	var root *node
	var pkgName string

	// Parse source files.
	for _, file := range files {
		name := file.Name()
		if skipFile(&interp.context, name, true) {
			continue
		}

		name = filepath.Join(dir, name)
		var buf []byte
		if buf, err = fs.ReadFile(interp.opt.filesystem, name); err != nil {
			return nil, err
		}

		n, err := interp.parse(string(buf), name, false)
		if err != nil {
			return nil, err
		}
		if n == nil {
			continue
		}

		var pname string
		if pname, root, err = interp.ast(n); err != nil {
			return nil, err
		}
		if root == nil {
			continue
		}

		if pkgName == "" {
			pkgName = pname
		} else if pkgName != pname {
			return nil, fmt.Errorf("found packages %s and %s in %s", pkgName, pname, dir)
		}
		rootNodes = append(rootNodes, root)

		subRPath := effectivePkg(rPath, importPath)
		var list []*node
		list, err = interp.gta(root, subRPath, importPath, pkgName)
		if err != nil {
			return nil, fmt.Errorf("gta failed:%w subRPath:%s importPath:%s pkgName:%s file:%s", err, subRPath, importPath, pkgName, file)
		}
		revisit[subRPath] = append(revisit[subRPath], list...)
	}

	// Revisit incomplete nodes where GTA could not complete.
	for _, nodes := range revisit {
		if err = interp.gtaRetry(nodes, importPath, pkgName); err != nil {
			return nil, err
		}
	}

	// Generate control flow graphs.
	for _, root := range rootNodes {
		var nodes []*node
		if nodes, err = interp.cfg(root, nil, importPath, pkgName); err != nil {
			return nil, err
		}
		initNodes = append(initNodes, nodes...)
	}

	// Register source package in the interpreter. The package contains only
	// the global symbols in the package scope.
	interp.mutex.Lock()
	gs := interp.scopes[importPath]
	if gs == nil {
		interp.mutex.Unlock()
		// A nil scope means that no even an empty package is created from source.
		return nil, fmt.Errorf("no Go files in %s", dir)
	}
	interp.srcPkg[importPath] = gs.sym
	interp.pkgNames[importPath] = pkgName

	interp.frame.mutex.Lock()
	interp.resizeFrame()
	interp.frame.mutex.Unlock()
	interp.mutex.Unlock()

	// Wire and execute global vars in global scope gs.
	n, err := genGlobalVars(rootNodes, gs)
	if err != nil {
		return nil, err
	}
	interp.run(n, nil)
	// if n != nil {
	// 	initNodes = append(initNodes, n)
	// } else {
	// 	slog.Debug("no node for global vars")
	// }
	// initNodes = append(rootNodes, initNodes...)

	// Add main to list of functions to run, after all inits.
	if m := gs.sym[mainID]; pkgName == mainID && m != nil {
		initNodes = append(initNodes, m.node)
	}

	// if debug enabled then export CFG and AST
	if slog.Default().Enabled(context.Background(), slog.LevelDebug) {
		exportDots(initNodes, root)
	}

	return &Program{
		pkgName: "main",
		init:    initNodes,
		root:    root,
	}, nil
}

// for inspection only
func exportDots(initNodes []*node, root *node) {
	cwd, _ := os.Getwd()
	indices := []int64{}
	for _, each := range initNodes {
		if each != nil { // can be nil
			indices = append(indices, each.index)
		}
	}
	slog.Debug("exporting CFG and AST",
		"root.index", root.index,
		"initnodes", indices,
		"cfg", fmt.Sprintf("dot -Tpng %s/varvoy-cfg.dot > varvoy-cfg.png && open varvoy-cfg.png", cwd),
		"ast", fmt.Sprintf("dot -Tpng %s/varvoy-ast.dot > varvoy-ast.png && open varvoy-ast.png", cwd))
	df, _ := os.Create("varvoy-cfg.dot")
	root.cfgDot(df)
	df.Close()
	df, _ = os.Create("varvoy-ast.dot")
	root.astDot(df, "ast")
	df.Close()
}
