package interp

import (
	"fmt"
	"io/fs"
	"path/filepath"
	"strconv"
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
			return nil, err
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

	// Once all package sources have been parsed, collect entry points then init functions.
	rootRunners := []*node{}
	for _, n := range rootNodes {
		if err = genRun(n); err != nil {
			return nil, err
		}
		// do not run it but save it for initnodes
		rootRunners = append(rootRunners, n)
	}
	initNodes = append(rootRunners, initNodes...)

	// Wire and execute global vars in global scope gs.
	n, err := genGlobalVars(rootNodes, gs)
	if err != nil {
		return nil, err
	}
	// do run these ahead of the program
	interp.run(n, nil)

	// Add main to list of functions to run, after all inits.
	if m := gs.sym[mainID]; pkgName == mainID && m != nil {
		initNodes = append(initNodes, m.node)
	}
	return &Program{
		pkgName: "main",
		init:    initNodes,
		root:    root,
	}, nil
}

func (b Breakpoint) String() string {
	return b.Position.String() + ",valid=" + strconv.FormatBool(b.Valid)
}
