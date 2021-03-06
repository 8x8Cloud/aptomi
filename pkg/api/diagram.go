package api

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/Aptomi/aptomi/pkg/engine/resolve"
	"github.com/Aptomi/aptomi/pkg/lang"
	"github.com/Aptomi/aptomi/pkg/runtime"
	"github.com/Aptomi/aptomi/pkg/visualization"
	"github.com/julienschmidt/httprouter"
)

type graphWrapper struct {
	Data interface{}
}

func (g *graphWrapper) GetKind() string {
	return "graph"
}

func (api *coreAPI) handlePolicyDiagram(writer http.ResponseWriter, request *http.Request, params httprouter.Params) {
	mode := params.ByName("mode")
	gen := params.ByName("gen")

	if len(gen) == 0 {
		gen = strconv.Itoa(int(runtime.LastOrEmptyGen))
	}

	// see which policy generation we need to load
	policyGen := runtime.ParseGeneration(gen)
	if strings.ToLower(mode) == "actual" {
		policyGen = runtime.LastOrEmptyGen
	}

	// load policy by gen
	policy, policyGen, err := api.registry.GetPolicy(policyGen)
	if err != nil {
		panic(fmt.Sprintf("error while getting requested policy: %s", err))
	}

	// load revision
	revision, err := api.registry.GetLastRevisionForPolicy(policyGen)
	if err != nil {
		panic(fmt.Sprintf("error while loading revision from the registry: %s", err))
	}

	// load desired state
	desiredState, err := api.registry.GetDesiredState(revision)
	if err != nil {
		panic(fmt.Sprintf("can't load desired state from revision: %s", err))
	}

	// load actual state
	actualState, err := api.registry.GetActualState()
	if err != nil {
		panic(fmt.Sprintf("can't load actual state: %s", err))
	}

	var graph *visualization.Graph
	switch strings.ToLower(mode) {
	case "policy":
		// show just policy
		graphBuilder := visualization.NewGraphBuilder(policy, nil, nil)
		graph = graphBuilder.Policy(visualization.PolicyCfgDefault)
	case "desired":
		// show instances in desired state
		graphBuilder := visualization.NewGraphBuilder(policy, desiredState, api.externalData)
		graph = graphBuilder.ClaimResolution(visualization.ClaimResolutionCfgDefault)
	case "actual":
		// TODO: actual may not work correctly in all cases (e.g. after policy delete on a cluster which is not available, desired state has less components, these components are still in actual state but will not be shown on UI)
		// show instances in actual state
		graphBuilder := visualization.NewGraphBuilder(policy, desiredState, api.externalData)
		graph = graphBuilder.ClaimResolutionWithFunc(visualization.ClaimResolutionCfgDefault, func(instance *resolve.ComponentInstance) bool {
			_, found := actualState.ComponentInstanceMap[instance.GetKey()]
			return found
		})
	default:
		panic("unknown mode: " + mode)
	}

	api.contentType.WriteOne(writer, request, &graphWrapper{Data: graph.GetData()})
}

func (api *coreAPI) handlePolicyDiagramCompare(writer http.ResponseWriter, request *http.Request, params httprouter.Params) {
	mode := params.ByName("mode")
	gen := params.ByName("gen")
	if len(gen) == 0 {
		gen = strconv.Itoa(int(runtime.LastOrEmptyGen))
	}

	genBase := params.ByName("genBase")
	if len(genBase) == 0 {
		genBase = strconv.Itoa(int(runtime.LastOrEmptyGen))
	}

	policy, policyGen, err := api.registry.GetPolicy(runtime.ParseGeneration(gen))
	if err != nil {
		panic(fmt.Sprintf("error while getting requested policy: %s", err))
	}
	policyBase, policyBaseGen, err := api.registry.GetPolicy(runtime.ParseGeneration(genBase))
	if err != nil {
		panic(fmt.Sprintf("error while getting requested policy: %s", err))
	}

	var graph *visualization.Graph
	switch strings.ToLower(mode) {
	case "policy":
		// policy & policy base
		graph = visualization.NewGraphBuilder(policy, nil, nil).Policy(visualization.PolicyCfgDefault)
		graphBase := visualization.NewGraphBuilder(policyBase, nil, nil).Policy(visualization.PolicyCfgDefault)

		// diff
		graph.CalcDelta(graphBase)
	case "desired":
		// desired state (next)
		{
			revision, err := api.registry.GetLastRevisionForPolicy(policyGen)
			if err != nil {
				panic(fmt.Sprintf("error while loading revision from the registry: %s", err))
			}

			desiredState, err := api.registry.GetDesiredState(revision)
			if err != nil {
				panic(fmt.Sprintf("can't load desired from revision: %s", err))
			}

			graphBuilder := visualization.NewGraphBuilder(policy, desiredState, api.externalData)
			graph = graphBuilder.ClaimResolution(visualization.ClaimResolutionCfgDefault)
		}

		// desired state (prev)
		var graphBase *visualization.Graph
		{
			revisionBase, err := api.registry.GetLastRevisionForPolicy(policyBaseGen)
			if err != nil {
				panic(fmt.Sprintf("error while loading revision from the registry: %s", err))
			}

			desiredStateBase, err := api.registry.GetDesiredState(revisionBase)
			if err != nil {
				panic(fmt.Sprintf("can't load desired state from revision: %s", err))
			}

			graphBuilderBase := visualization.NewGraphBuilder(policyBase, desiredStateBase, api.externalData)
			graphBase = graphBuilderBase.ClaimResolution(visualization.ClaimResolutionCfgDefault)
		}

		// diff
		graph.CalcDelta(graphBase)
	default:
		panic("unknown mode: " + mode)
	}

	api.contentType.WriteOne(writer, request, &graphWrapper{Data: graph.GetData()})
}

func (api *coreAPI) handleObjectDiagram(writer http.ResponseWriter, request *http.Request, params httprouter.Params) {
	ns := params.ByName("ns")
	kind := params.ByName("kind")
	name := params.ByName("name")

	policy, policyGen, err := api.registry.GetPolicy(runtime.LastOrEmptyGen)
	if err != nil {
		panic(fmt.Sprintf("error while getting policy: %s", err))
	}

	obj, err := policy.GetObject(kind, name, ns)
	if err != nil {
		panic(fmt.Sprintf("error while getting object from policy: %s", err))
	}

	var desiredState *resolve.PolicyResolution
	if kind == lang.TypeClaim.Kind {
		// load revision
		revision, err := api.registry.GetLastRevisionForPolicy(policyGen)
		if err != nil {
			panic(fmt.Sprintf("error while loading revision from the registry: %s", err))
		}

		// load desired state
		desiredState, err = api.registry.GetDesiredState(revision)
		if err != nil {
			panic(fmt.Sprintf("can't load desired state from revision: %s", err))
		}
	}

	graphBuilder := visualization.NewGraphBuilder(policy, desiredState, api.externalData)
	graph := graphBuilder.Object(obj)

	api.contentType.WriteOne(writer, request, &graphWrapper{Data: graph.GetData()})
}
