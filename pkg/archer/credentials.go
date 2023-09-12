package archer

import (
	"github.com/gophercloud/gophercloud"
	"github.com/gophercloud/utils/openstack/clientconfig"
)

// LoadServiceClient loads the service client for the endpoint-services service.
func LoadServiceClient() (*gophercloud.ServiceClient, error) {
	serviceType := "endpoint-services"

	provider, err := clientconfig.AuthenticatedClient(nil)
	if err != nil {
		return nil, err
	}
	eo := gophercloud.EndpointOpts{}
	eo.ApplyDefaults(serviceType)
	// Override endpoint?
	var url string
	if url, err = provider.EndpointLocator(eo); err != nil {
		return nil, err
	}

	sc := &gophercloud.ServiceClient{
		ProviderClient: provider,
		Endpoint:       url,
		Type:           serviceType,
	}
	return sc, nil
}
