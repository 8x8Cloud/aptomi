package resolve

import (
	"github.com/Aptomi/aptomi/pkg/slinga/lang/builder"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestComponentKeyCopy(t *testing.T) {
	// create component key
	b := builder.NewPolicyBuilder()
	service := b.AddService(b.AddUser())
	b.AddServiceComponent(service, b.CodeComponent(nil, nil))
	contract := b.AddContract(service, b.CriteriaTrue())
	key := NewComponentInstanceKey(
		b.AddCluster(),
		contract,
		contract.Contexts[0],
		[]string{"x", "y", "z"},
		service,
		service.Components[0],
	)

	// make component key copy
	keyCopy := key.MakeCopy()

	// check that both keys as strings are identical
	assert.Equal(t, key.GetKey(), keyCopy.GetKey(), "Component key should be copied successfully")
}
