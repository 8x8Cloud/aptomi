package engine

import (
	. "github.com/Aptomi/aptomi/pkg/slinga/language"
	"github.com/stretchr/testify/assert"
	"testing"
	"time"
	"strconv"
)

func BenchmarkEngine(b *testing.B) {
	t := &testing.T{}
	for i := 0; i < b.N; i++ {
		TestPolicyResolve(t)
		TestPolicyResolveEmptyDiff(t)
		TestPolicyResolveNonEmptyDiff(t)
		TestDiffUpdateAndComponentTimes(t)
	}
}

func TestPolicyResolve(t *testing.T) {
	policy := loadUnitTestsPolicy()
	userLoader := NewUserLoaderFromDir("../testdata/unittests")

	usageState := NewServiceUsageState(policy, userLoader)
	err := usageState.ResolveAllDependencies()
	resolvedUsage := usageState.GetResolvedData()

	// Check that policy resolution finished correctly
	assert.Nil(t, err, "Policy usage should be resolved without errors")
	assert.NotEqual(t, 0, len(resolvedUsage.ComponentProcessingOrder), "Policy usage should have entries")

	kafkaTest := resolvedUsage.ComponentInstanceMap["kafka#test#test-platform_services#component2"]
	kafkaProd := resolvedUsage.ComponentInstanceMap["kafka#prod-low#prod-team-platform_services-true#component2"]
	assert.Equal(t, 1, len(kafkaTest.DependencyIds), "One dependency should be resolved with access to test")
	assert.True(t, policy.Dependencies.DependenciesByID["dep_id_1"].Resolved, "Only Alice should have access to test")
	assert.False(t, policy.Dependencies.DependenciesByID["dep_id_4"].Resolved, "Partial matching is broken. User has access to kafka, but not to zookeeper that kafka depends on. This should not be resolved successfully")

	assert.Equal(t, 1, len(kafkaProd.DependencyIds), "One dependency should be resolved with access to prod")
	assert.Equal(t, "2", policy.Dependencies.DependenciesByID["dep_id_2"].UserID, "Only Bob should have access to prod (Carol is compromised)")

	// Check labels for Bob's dependency
	key := policy.Dependencies.DependenciesByID["dep_id_2"].ServiceKey
	labels := usageState.ResolvedData.ComponentInstanceMap[key].CalculatedLabels.Labels
	assert.Equal(t, "yes", labels["important"], "Label 'important=yes' should be carried from dependency all the way through the policy")
	assert.Equal(t, "true", labels["prod-low-ctx"], "Label 'prod-low-ctx=true' should be added on context match")
	assert.Equal(t, "", labels["some-label-to-be-removed"], "Label 'some-label-to-be-removed' should be removed on context match")
	assert.Equal(t, "true", labels["prod-low-alloc"], "Label 'prod-low-alloc=true' should be added on allocation match")

	// Check that code parameters evaluate correctly
	assert.Equal(t, "zookeeper-test-test-platform-services-component2", kafkaTest.CalculatedCodeParams["address"], "Code parameter should be calculated correctly")

	// Check that discovery parameters evaluate correctly
	assert.Equal(t, "kafka-kafka-test-test-platform-services-component2-url", kafkaTest.CalculatedDiscovery["url"], "Discovery parameter should be calculated correctly")

	// Check that nested parameters evaluate correctly
	for i := 1; i <= 5; i++ {
		assert.Equal(t, "value"+strconv.Itoa(i), kafkaTest.CalculatedCodeParams.GetNestedMap("data" + strconv.Itoa(i)).GetNestedMap("param")["name"], "Nested code parameters should be calculated correctly")
	}
}

func TestPolicyResolveEmptyDiff(t *testing.T) {
	policy := loadUnitTestsPolicy()
	userLoader := NewUserLoaderFromDir("../testdata/unittests")

	// Get usage state prev and emulate save/load
	usageStatePrev := NewServiceUsageState(policy, userLoader)
	usageStatePrev.ResolveAllDependencies()
	usageStatePrev = emulateSaveAndLoadState(usageStatePrev)

	// Get usage state next
	usageStateNext := NewServiceUsageState(policy, userLoader)
	usageStateNext.ResolveAllDependencies()

	// Calculate difference
	diff := usageStateNext.CalculateDifference(&usageStatePrev)

	assert.False(t, diff.ShouldGenerateNewRevision(), "Diff should not have any changes")
	assert.Equal(t, 0, len(diff.ComponentInstantiate), "Empty diff should not have any component instantiations")
	assert.Equal(t, 0, len(diff.ComponentDestruct), "Empty diff should not have any component destructions")
	assert.Equal(t, 0, len(diff.ComponentUpdate), "Empty diff should not have any component updates")
	assert.Equal(t, 0, len(diff.ComponentAttachDependency), "Empty diff should not have any dependencies attached to components")
	assert.Equal(t, 0, len(diff.ComponentDetachDependency), "Empty diff should not have any dependencies removed from components")
}

func TestPolicyResolveNonEmptyDiff(t *testing.T) {
	policyPrev := loadUnitTestsPolicy()
	userLoader := NewUserLoaderFromDir("../testdata/unittests")

	// Get usage state prev and emulate save/load
	usageStatePrev := NewServiceUsageState(policyPrev, userLoader)
	usageStatePrev.ResolveAllDependencies()
	usageStatePrev = emulateSaveAndLoadState(usageStatePrev)

	// Add another dependency and resolve usage state next
	policyNext := loadUnitTestsPolicy()
	policyNext.Dependencies.AddDependency(
		&Dependency{
			SlingaObject: &SlingaObject{Metadata: SlingaObjectMetadata{Namespace: "main", Name: "dep_id_5"}},
			UserID:       "5",
			Service:      "kafka",
		},
	)
	usageStateNext := NewServiceUsageState(policyNext, userLoader)
	usageStateNext.ResolveAllDependencies()

	// Calculate difference
	diff := usageStateNext.CalculateDifference(&usageStatePrev)

	assert.True(t, diff.ShouldGenerateNewRevision(), "Diff should have changes")
	assert.Equal(t, 7, len(diff.ComponentInstantiate), "Diff should have component instantiations")
	assert.Equal(t, 0, len(diff.ComponentDestruct), "Diff should not have any component destructions")
	assert.Equal(t, 0, len(diff.ComponentUpdate), "Diff should not have any component updates")
	assert.Equal(t, 7, len(diff.ComponentAttachDependency), "Diff should have 7 dependencies attached to components")
	assert.Equal(t, 0, len(diff.ComponentDetachDependency), "Diff should not have any dependencies removed from components")
}

func TestDiffUpdateAndComponentTimes(t *testing.T) {
	policyPrev := loadUnitTestsPolicy()
	userLoader := NewUserLoaderFromDir("../testdata/unittests")

	var key string
	var timePrevCreated, timePrevUpdated, timeNextCreated, timeNextUpdated time.Time

	// Create initial empty state (do not resolve any dependencies)
	uEmpty := NewServiceUsageState(policyPrev, userLoader)

	// Resolve, calculate difference against empty state, emulate save/load
	uInitial := NewServiceUsageState(policyPrev, userLoader)
	uInitial.ResolveAllDependencies()
	uInitial.CalculateDifference(&uEmpty)
	uInitial = emulateSaveAndLoadState(uInitial)

	// Check creation/update times
	key = "kafka#test#test-platform_services#component2"
	timeNextCreated = uInitial.ResolvedData.ComponentInstanceMap[key].CreatedOn
	timeNextUpdated = uInitial.ResolvedData.ComponentInstanceMap[key].UpdatedOn
	assert.WithinDuration(t, time.Now(), timeNextCreated, time.Second, "Creation time should be initialized correctly for kafka")
	assert.Equal(t, timeNextUpdated, timeNextUpdated, "Update time should be equal to creation time")

	// Sleep a little bit to introduce time delay
	time.Sleep(25 * time.Millisecond)

	// Add another dependency, resolve, calculate difference against prev state, emulate save/load
	policyNext := loadUnitTestsPolicy()
	policyNext.Dependencies.AddDependency(
		&Dependency{
			SlingaObject: &SlingaObject{Metadata: SlingaObjectMetadata{Namespace: "main", Name: "dep_id_5"}},
			UserID:       "5",
			Service:      "kafka",
		},
	)
	uNewDependency := NewServiceUsageState(policyNext, userLoader)
	uNewDependency.ResolveAllDependencies()
	uNewDependency.CalculateDifference(&uInitial)

	// Check creation/update times
	timePrevCreated = uInitial.ResolvedData.ComponentInstanceMap[key].CreatedOn
	timePrevUpdated = uInitial.ResolvedData.ComponentInstanceMap[key].UpdatedOn
	timeNextCreated = uNewDependency.ResolvedData.ComponentInstanceMap[key].CreatedOn
	timeNextUpdated = uNewDependency.ResolvedData.ComponentInstanceMap[key].UpdatedOn
	assert.Equal(t, timePrevCreated, timeNextCreated, "Creation time should be carried over to remain the same")
	assert.Equal(t, timePrevUpdated, timeNextUpdated, "Update time should be carried over to remain the same")

	// Sleep a little bit to introduce time delay
	time.Sleep(25 * time.Millisecond)

	// Update user label, re-evaluate and see that component instance has changed
	userLoader.LoadUserByID("5").Labels["changinglabel"] = "newvalue"
	uUpdatedDependency := NewServiceUsageState(policyNext, userLoader)
	uUpdatedDependency.ResolveAllDependencies()
	diff := uUpdatedDependency.CalculateDifference(&uNewDependency)

	// Check that update has been performed
	assert.True(t, diff.ShouldGenerateNewRevision(), "Diff should have changes")
	assert.Equal(t, 0, len(diff.ComponentInstantiate), "Diff should not have component instantiations")
	assert.Equal(t, 0, len(diff.ComponentDestruct), "Diff should not have any component destructions")
	assert.Equal(t, 1, len(diff.ComponentUpdate), "Diff should have component update")
	assert.Equal(t, 0, len(diff.ComponentAttachDependency), "Diff should not have any dependencies attached to components")
	assert.Equal(t, 0, len(diff.ComponentDetachDependency), "Diff should not have any dependencies removed from components")

	// Check creation/update times for component
	key = "kafka#prod-high#prod-Elena#component2"
	timePrevCreated = uNewDependency.ResolvedData.ComponentInstanceMap[key].CreatedOn
	timePrevUpdated = uNewDependency.ResolvedData.ComponentInstanceMap[key].UpdatedOn
	timeNextCreated = uUpdatedDependency.ResolvedData.ComponentInstanceMap[key].CreatedOn
	timeNextUpdated = uUpdatedDependency.ResolvedData.ComponentInstanceMap[key].UpdatedOn
	assert.Equal(t, timePrevCreated, timeNextCreated, "Creation time should be carried over to remain the same")
	assert.True(t, timeNextUpdated.After(timePrevUpdated), "Update time should be changed")

	// Check creation/update times for service
	key = "kafka#prod-high#prod-Elena#root"
	timePrevCreated = uNewDependency.ResolvedData.ComponentInstanceMap[key].CreatedOn
	timePrevUpdated = uNewDependency.ResolvedData.ComponentInstanceMap[key].UpdatedOn
	timeNextCreated = uUpdatedDependency.ResolvedData.ComponentInstanceMap[key].CreatedOn
	timeNextUpdated = uUpdatedDependency.ResolvedData.ComponentInstanceMap[key].UpdatedOn
	assert.Equal(t, timePrevCreated, timeNextCreated, "Creation time should be carried over to remain the same")
	assert.True(t, timeNextUpdated.After(timePrevUpdated), "Update time should be changed for service")
}

func TestServiceComponentsTopologicalOrder(t *testing.T) {
	policy := loadUnitTestsPolicy()
	service := policy.Services["kafka"]

	c, err := service.GetComponentsSortedTopologically()
	assert.Nil(t, err, "Service components should be topologically sorted without errors")

	assert.Equal(t, len(c), 3, "Component topological sort should produce correct number of values")
	assert.Equal(t, "component1", c[0].Name, "Component topological sort should produce correct order")
	assert.Equal(t, "component2", c[1].Name, "Component topological sort should produce correct order")
	assert.Equal(t, "component3", c[2].Name, "Component topological sort should produce correct order")
}