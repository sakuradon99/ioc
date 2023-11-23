package ioc

import "fmt"

type Interface struct {
	id            string
	implObjectIDs []string
}

type InterfacePool struct {
	idToInterface map[string]Interface
}

func NewInterfacePool() *InterfacePool {
	return &InterfacePool{
		idToInterface: make(map[string]Interface),
	}
}

func (p *InterfacePool) Add(inf Interface) {
	if _, ok := p.idToInterface[inf.id]; ok {
		return
	}
	p.idToInterface[inf.id] = inf
}

func (p *InterfacePool) BindImplement(interfaceID, objectID string) error {
	inf, ok := p.idToInterface[interfaceID]
	if !ok {
		return fmt.Errorf("interface with id %s not found", interfaceID)
	}
	inf.implObjectIDs = append(inf.implObjectIDs, objectID)
	p.idToInterface[interfaceID] = inf
	return nil
}

func (p *InterfacePool) GetImplementObjectIDs(interfaceID string) ([]string, error) {
	inf, ok := p.idToInterface[interfaceID]
	if !ok {
		return nil, fmt.Errorf("interface with id %s not found", interfaceID)
	}

	return inf.implObjectIDs, nil
}

func genInterfaceID(pkgPath string, name string) string {
	return fmt.Sprintf("%s.%s", pkgPath, name)
}
