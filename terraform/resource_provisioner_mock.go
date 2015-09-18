package terraform

import "sync"

// MockResourceProvisioner implements ResourceProvisioner but mocks out all the
// calls for testing purposes.
type MockResourceProvisioner struct {
	sync.Mutex
	// Anything you want, in case you need to store extra data with the mock.
	Meta interface{}

	ApplyCalled      bool
	ApplyOutput      UIOutput
	ApplyState       *InstanceState
	ApplyConfig      *ResourceConfig
	ApplyFn          func(*InstanceState, *ResourceConfig) error
	ApplyReturnError error

	ValidateCalled       bool
	ValidateConfig       *ResourceConfig
	ValidateFn           func(c *ResourceConfig) ([]string, []error)
	ValidateReturnWarns  []string
	ValidateReturnErrors []error

	StopCalled      bool
	StopFn          func() error
	StopReturnError error

	ExportCalled      bool
	ExportFn          func() (*ResourceProvisionerSchema, error)
	ExportReturn      *ResourceProvisionerSchema
	ExportReturnError error
}

func (p *MockResourceProvisioner) Validate(c *ResourceConfig) ([]string, []error) {
	p.Lock()
	defer p.Unlock()

	p.ValidateCalled = true
	p.ValidateConfig = c
	if p.ValidateFn != nil {
		return p.ValidateFn(c)
	}
	return p.ValidateReturnWarns, p.ValidateReturnErrors
}

func (p *MockResourceProvisioner) Apply(
	output UIOutput,
	state *InstanceState,
	c *ResourceConfig) error {
	p.Lock()

	p.ApplyCalled = true
	p.ApplyOutput = output
	p.ApplyState = state
	p.ApplyConfig = c
	if p.ApplyFn != nil {
		fn := p.ApplyFn
		p.Unlock()
		return fn(state, c)
	}

	defer p.Unlock()
	return p.ApplyReturnError
}

func (p *MockResourceProvisioner) Stop() error {
	p.Lock()
	defer p.Unlock()

	p.StopCalled = true
	if p.StopFn != nil {
		return p.StopFn()
	}

	return p.StopReturnError
}

func (p *MockResourceProvisioner) Export() (*ResourceProvisionerSchema, error) {
	p.ExportCalled = true
	if p.ExportFn != nil {
		return p.ExportFn()
	}

	return p.ExportReturn, p.ExportReturnError
}
