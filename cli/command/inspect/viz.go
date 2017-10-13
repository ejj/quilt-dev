package inspect

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"github.com/kelda/kelda/util"
)

func stripExtension(configPath string) string {
	ext := filepath.Ext(configPath)
	return strings.TrimSuffix(configPath, ext)
}

func viz(configPath string, graph Graph, outputFormat string) {
	slug := stripExtension(configPath)
	dot := makeGraphviz(graph)
	graphviz(outputFormat, slug, dot)
}

func makeGraphviz(graph Graph) string {
	var nodes []string
	for node := range graph.Nodes {
		nodes = append(nodes, fmt.Sprintf("    %q;", node))
	}
	sort.Strings(nodes)

	var connections []string
	for _, edge := range graph.GetConnections() {
		connections = append(connections,
			fmt.Sprintf(
				"    %q -> %q;",
				edge.From,
				edge.To,
			),
		)
	}
	sort.Strings(connections)

	return "strict digraph {\n" +
		strings.Join(nodes, "\n") + "\n" +
		strings.Join(connections, "\n") + "\n" +
		"}\n"
}

// Graphviz generates a specification for the graphviz program that visualizes the
// communication graph of a blueprint.
func graphviz(outputFormat string, slug string, dot string) {
	f, err := util.AppFs.Create(slug + ".dot")
	if err != nil {
		panic(err)
	}
	defer f.Close()
	f.Write([]byte(dot))
	if outputFormat == "graphviz" {
		return
	}
	defer exec.Command("rm", slug+".dot").Run()

	// Dependencies:
	// - easy-graph (install Graph::Easy from cpan)
	// - graphviz (install from your favorite package manager)
	var writeGraph *exec.Cmd
	switch outputFormat {
	case "ascii":
		writeGraph = exec.Command("graph-easy", "--input="+slug+".dot",
			"--as_ascii")
	case "pdf":
		writeGraph = exec.Command("dot", "-Tpdf", "-o", slug+".pdf",
			slug+".dot")
	}
	writeGraph.Stdout = os.Stdout
	writeGraph.Stderr = os.Stderr
	writeGraph.Run()
}
