package phab

import (
	"fmt"
	"sort"
	"strings"

	"github.com/etcinit/gonduit/entities"
)

type SearchConstaints struct {
	Projects []string `json:"projects"`
}

type ManifestSearch struct {
	Constraints SearchConstaints `json:"constraints"`
}

type TaskTree struct {
	*entities.ManiphestTask
	Items []*TaskTree
}

func StringTree(t *TaskTree) (result string) {
	result += fmt.Sprintf("%s: %s\n", t.ObjectName, t.Title)
	var spaces []bool
	result += stringObjItems(t.Items, spaces)
	return result
}

func stringLine(name string, spaces []bool, last bool) (result string) {
	for _, space := range spaces {
		if space {
			result += "    "
		} else {
			result += "│   "
		}
	}

	indicator := "├── "
	if last {
		indicator = "└── "
	}

	result += indicator + name + "\n"
	return
}

func stringObjItems(items []*TaskTree, spaces []bool) (result string) {
	sort.Slice(items, func(i, j int) bool { return items[i].ID < items[j].ID })

	for i, f := range items {
		last := (i >= len(items)-1)
		result += stringLine(fmt.Sprintf("%s: %-6v - %s", f.ObjectName, strings.ToUpper(f.Priority), f.Title), spaces, last)
		if len(f.Items) > 0 {
			spacesChild := append(spaces, last)
			result += stringObjItems(f.Items, spacesChild)
		}
	}
	return
}
