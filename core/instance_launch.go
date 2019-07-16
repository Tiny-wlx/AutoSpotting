package autospotting

import (
	"encoding/json"
	"errors"
	"fmt"

	"github.com/aws/aws-lambda-go/events"
)

// GetPendingReplaceableOnDemandInstance return a populated Instance data structure related
// to an on-demand instance that has just been launched for an AutoScaling Group
// managed by AutoSpotting and just entered the Pending state.
func GetPendingReplaceableOnDemandInstance(event events.CloudWatchEvent, cfg *Config) (*Instance, error) {
	regionName, instanceID, err := GetPendingInstanceID(event)
	if err != nil {
		logger.Println("Couldn't determine instance ID", err.Error())
		return nil, err
	}

	r := region{name: *regionName, conf: cfg}
	if !r.enabled() {
		return nil, fmt.Errorf("region %s is not enabled", *regionName)
	}

	r.services.connect(*regionName)
	r.setupAsgFilters()
	r.scanForEnabledAutoScalingGroups()

	if err := r.scanInstance(instanceID); err != nil {
		logger.Printf("%s Couldn't scan instance %s: %s", *regionName,
			*instanceID, err.Error())
		return nil, err
	}

	i := r.instances.get(*instanceID)

	if !i.shouldBeReplacedWithSpot() {
		logger.Printf("%s Instance %s is not supposed to be replaced, skipping...",
			*regionName, *instanceID)
		return nil, errors.New("instance not supposed to be replaced")
	}

	return i, nil
}

//GetPendingInstanceID checks if the given CloudWatch event data is triggered
//from an instance recently launched and still in pending state. It returns the
//instance ID present in the event data
func GetPendingInstanceID(event events.CloudWatchEvent) (*string, *string, error) {

	var detailData instanceData
	if err := json.Unmarshal(event.Detail, &detailData); err != nil {
		logger.Println(err.Error())
		return nil, nil, err
	}

	if detailData.State != nil && *detailData.State == "pending" {
		return &event.Region, detailData.InstanceID, nil
	}

	logger.Println("This code shouldn't be reachable")
	return nil, nil, errors.New("this code shoudn't be reached")
}
