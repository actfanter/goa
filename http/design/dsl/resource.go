package dsl

// Resource describes a set of related endpoints, if implementing a REST API then it describes a
// single REST resource.
//
// The resource DSL allows listing the supported resource actions. Each action corresponds to a
// single API endpoint. See Action.
//
// The resource DSL also allows setting the resource default media type. This media
// type is used to render the response body of actions that return the OK response (unless the
// action overrides the default). The default media type also sets the properties of the request
// payload attributes with the same name. See DefaultMedia.
//
// The resource DSL can also specify a parent resource. Defining a parent resources has two effects.
// First, parent resources set the prefix of all resource action paths to the parent resource href.
// Note that actions can override the path using an absolute path (that is a path starting with
// "//").  Second, goa uses the parent resource href coupled with the resource BasePath if any to
// build request paths.
//
// By default goa uses the show action if present to compute a resource href (basically
// concatenating the parent resource href with the base path and show action path). The resource
// definition may specify a canonical action via CanonicalActionName to override that default.
//
//
// Resource is a top level DSL.
//
// Resource accepts two arguments: the name of the resource and its defining API.
//
// Example:
//
//     Resource("bottle", func() {
//         Description("A wine bottle")    // Resource description
//         DefaultMedia(BottleMedia)       // Resource default media type if any
//         BasePath("/bottles")            // Common resource action path prefix if any
//         Parent("account")               // Name of parent resource if any
//         CanonicalActionName("get")      // Name of action that used to compute
//                                         // href if not "show"
//         UseTrait("Authenticated")       // Included trait, can appear more than once
//
//         Response(Unauthorized, ErrorMedia) // Common responses to all actions
//         Response(BadRequest, ErrorMedia)   // can appear more than once
//
//         Action("show", func() {        // Action definition, can appear more than once
//             // ... Action dsl
//         })
//     })
//
func Resource(name string, dsl func()) *design.ResourceDefinition {
	if design.Design.Resources == nil {
		design.Design.Resources = make(map[string]*design.ResourceDefinition)
	}
	if !dslengine.IsTopLevelDefinition() {
		dslengine.IncompatibleDSL()
		return nil
	}

	if _, ok := design.Design.Resources[name]; ok {
		dslengine.ReportError("resource %#v is defined twice", name)
		return nil
	}
	resource := design.NewResourceDefinition(name, dsl)
	design.Design.Resources[name] = resource
	return resource
}
