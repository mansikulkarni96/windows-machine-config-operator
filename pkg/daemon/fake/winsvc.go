//go:build windows

package fake

import (
	"fmt"

	"golang.org/x/sys/windows/svc"
	"golang.org/x/sys/windows/svc/mgr"
)

type FakeService struct {
	name        string
	config      mgr.Config
	status      svc.Status
	serviceList *fakeServiceList
}

func (f *FakeService) Close() error {
	return nil
}

func (f *FakeService) Start(_ ...string) error {
	if f.status.State == svc.Running {
		return fmt.Errorf("service already running")
	}
	// each of the service's dependencies must be started before the service is started
	for _, dependency := range f.config.Dependencies {
		dependencyService, present := f.serviceList.read(dependency)
		if !present {
			return fmt.Errorf("dependent service doesnt exist")
		}
		// Windows will attempt to start the service only if it is not already running
		dependencyStatus, err := dependencyService.Query()
		if err != nil {
			return err
		}
		if dependencyStatus.State != svc.Running {
			err = dependencyService.Start()
			if err != nil {
				return fmt.Errorf("error starting dependency %s: %w", dependency, err)
			}
		}
	}
	f.status.State = svc.Running
	return nil
}

func (f *FakeService) Config() (mgr.Config, error) {
	return f.config, nil
}

func (f *FakeService) Control(cmd svc.Cmd) (svc.Status, error) {
	switch cmd {
	case svc.Stop:
		if f.status.State == svc.Stopped {
			return svc.Status{}, fmt.Errorf("service already stopped")
		}
		// Windows has a hard time stopping services that other services are dependent on. To most safely model this
		// functionality it is better to make it so that our mock manager is completely unable to stop services in that
		// scenario.
		existingServices := f.serviceList.listServiceNames()
		for _, serviceName := range existingServices {
			service, present := f.serviceList.read(serviceName)
			if !present {
				return svc.Status{}, fmt.Errorf("unable to open service %s", serviceName)
			}
			config, err := service.Config()
			if err != nil {
				return svc.Status{}, fmt.Errorf("error getting %s service config: %w", serviceName, err)
			}
			for _, dependency := range config.Dependencies {
				// Found a service that has this one as a dependency, ensure it is not running
				if dependency == f.name {
					status, err := service.Query()
					if err != nil {
						return svc.Status{}, fmt.Errorf("error querying %s service status: %w", serviceName, err)
					}
					if status.State != svc.Stopped {
						return svc.Status{}, fmt.Errorf("cannot stop service: %s, as other service is dependent on"+
							"it", serviceName)
					}
				}
			}
		}
		f.status.State = svc.Stopped
	}
	return f.status, nil
}

func (f *FakeService) Query() (svc.Status, error) {
	return f.status, nil
}

func (f *FakeService) UpdateConfig(config mgr.Config) error {
	f.config = config
	return nil
}

func (f *FakeService) ListDependentServices(_ svc.ActivityStatus) ([]string, error) {
	var dependencies []string
	for _, listedService := range f.serviceList.listServiceNames() {
		if listedService == f.name {
			continue
		}
		svc, found := f.serviceList.read(listedService)
		if !found {
			return nil, fmt.Errorf("unable to open service %s", listedService)
		}
		config, err := svc.Config()
		if err != nil {
			return nil, err
		}
		for _, s := range config.Dependencies {
			if s == f.name {
				dependencies = append(dependencies, listedService)
			}
		}
	}
	return dependencies, nil
}

func NewFakeService(name string, config mgr.Config, status svc.Status) *FakeService {
	return &FakeService{
		name:   name,
		config: config,
		status: status,
	}
}
