package globalid

import (
	"errors"
	"strings"
)

//
// IDs
//

// Components describes the components of a global identifier.
//
// When represented as a string the ID appears in the follwoing format, parens
// represent optional components.
//
//   srn:resource:(?org:)(?env:)(?resourceType/)uniqueComponents
//
// Example global IDs
//
//   srn:entities:default:default:agent/selene.local
//   srn:checks:auto:staging:disk-full
//   srn:users:deanlearner
//
type Components interface {
	// Resource definition associated with this ID.
	Resource() string

	// Organization is the organization the resource belongs to.
	Organization() string

	// Environment is the environment the resource belongs to.
	Environment() string

	// ResourceType is a optional element that describes any sort of sub-type of
	// the resource.
	ResourceType() string

	// UniqueComponents are component(s) of a resource that when combined uniquely
	// identify a resource.
	UniqueComponents() []string

	// String return string representation of ID
	String() string
}

// StandardComponents describes the standard components of a global identifier.
type StandardComponents struct {
	resource         string
	organization     string
	environment      string
	resourceType     string
	uniqueComponents []string
}

// String returns the string representation of the global ID.
func (id StandardComponents) String() string {
	var pathComponents []string
	var nameComponents []string

	uniqueComponents := strings.Join(id.uniqueComponents, "")
	nameComponents = append([]string{id.resourceType}, uniqueComponents)
	nameComponents = omitEmpty(nameComponents)
	pathComponents = omitEmpty([]string{
		id.resource,
		id.organization,
		id.environment,
	})

	// srn:{pathComponents}:{nameComponents}
	return "srn:" + strings.Join(pathComponents, ":") +
		":" + strings.Join(nameComponents, "/")
}

// Resource definition associated with this ID.
func (id StandardComponents) Resource() string {
	return id.resource
}

// Organization is the organization the resource belongs to.
func (id StandardComponents) Organization() string {
	return id.organization
}

// Environment is the environment the resource belongs to.
func (id StandardComponents) Environment() string {
	return id.environment
}

// ResourceType is a optional element that describes any sort of sub-type of
// the resource.
func (id StandardComponents) ResourceType() string {
	return id.resourceType
}

// UniqueComponents are component(s) of a resource that when combined uniquely
// identify a resource.
func (id StandardComponents) UniqueComponents() []string {
	return id.uniqueComponents
}

// Parse takes a global ID string, decodes it and returns it's components.
func Parse(gid string) (StandardComponents, error) {
	id := StandardComponents{}
	pathComponents := strings.Split(gid, ":")

	// Should be at least srn:resource:name
	if len(pathComponents) < 3 {
		return id, errors.New("given global ID does not appear valid")
	}

	// Pop the resource from the path components, eg. srn:resource:org:env:type/name
	//                                                    ^^^^^^^^
	id.resource = pathComponents[1]
	pathComponents = pathComponents[2:]

	// Pop the name components from the path components, eg. org:env:type/name
	//                                                               ^^^^^^^^^
	nameComponents := pathComponents[len(pathComponents)-2:]
	pathComponents = pathComponents[0 : len(pathComponents)-2]

	// If present pop the org from the path components, eg. org:env
	//                                                      ^^^
	if len(pathComponents) > 0 {
		id.organization = pathComponents[0]
		pathComponents = pathComponents[1:]
	}

	// If present pop the env from the path components, eg. env
	//                                                      ^^^
	if len(pathComponents) > 0 {
		id.environment = pathComponents[0]
	}

	// If present pop the type from the name components, eg. type/my-great-check
	//                                                       ^^^^
	if len(nameComponents) > 1 {
		id.resourceType = nameComponents[0]
		nameComponents = nameComponents[1:]
	}

	// Pop the remaining elements from the name components, eg. my-great-check
	//                                                          ^^^^^^^^^^^^^^
	id.uniqueComponents = nameComponents

	return id, nil
}

func omitEmpty(in []string) (out []string) {
	for _, n := range in {
		if n != "" {
			out = append(out, n)
		}
	}

	return
}
