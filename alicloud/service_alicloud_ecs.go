package alicloud

import (
	"encoding/json"
	"fmt"
	"github.com/denverdino/aliyungo/common"
	"github.com/denverdino/aliyungo/ecs"
	"github.com/hashicorp/terraform/helper/schema"
	"strings"
)

func (client *AliyunClient) DescribeImage(imageId string) (*ecs.ImageType, error) {

	pagination := common.Pagination{
		PageNumber: 1,
	}
	args := ecs.DescribeImagesArgs{
		Pagination: pagination,
		RegionId:   client.Region,
		Status:     ecs.ImageStatusAvailable,
	}

	var allImages []ecs.ImageType

	for {
		images, _, err := client.ecsconn.DescribeImages(&args)
		if err != nil {
			break
		}

		if len(images) == 0 {
			break
		}

		allImages = append(allImages, images...)

		args.Pagination.PageNumber++
	}

	if len(allImages) == 0 {
		return nil, common.GetClientErrorFromString("Not found")
	}

	var image *ecs.ImageType
	imageIds := []string{}
	for _, im := range allImages {
		if im.ImageId == imageId {
			image = &im
		}
		imageIds = append(imageIds, im.ImageId)
	}

	if image == nil {
		return nil, fmt.Errorf("image_id %s not exists in range %s, all images are %s", imageId, client.Region, imageIds)
	}

	return image, nil
}

// DescribeZone validate zoneId is valid in region
func (client *AliyunClient) DescribeZone(zoneID string) (*ecs.ZoneType, error) {
	zones, err := client.ecsconn.DescribeZones(client.Region)
	if err != nil {
		return nil, fmt.Errorf("error to list zones not found")
	}

	var zone *ecs.ZoneType
	zoneIds := []string{}
	for _, z := range zones {
		if z.ZoneId == zoneID {
			zone = &ecs.ZoneType{
				ZoneId:                    z.ZoneId,
				LocalName:                 z.LocalName,
				AvailableResourceCreation: z.AvailableResourceCreation,
				AvailableDiskCategories:   z.AvailableDiskCategories,
			}
		}
		zoneIds = append(zoneIds, z.ZoneId)
	}

	if zone == nil {
		return nil, fmt.Errorf("availability_zone not exists in range %s, all zones are %s", client.Region, zoneIds)
	}

	return zone, nil
}

// return multiIZ list of current region
func (client *AliyunClient) DescribeMultiIZByRegion() (izs []string, err error) {
	resp, err := client.rdsconn.DescribeRegions()
	if err != nil {
		return nil, fmt.Errorf("error to list regions not found")
	}
	regions := resp.Regions.RDSRegion

	zoneIds := []string{}
	for _, r := range regions {
		if r.RegionId == string(client.Region) && strings.Contains(r.ZoneId, MULTI_IZ_SYMBOL) {
			zoneIds = append(zoneIds, r.ZoneId)
		}
	}

	return zoneIds, nil
}

func (client *AliyunClient) QueryInstancesByIds(ids []string) (instances []ecs.InstanceAttributesType, err error) {
	idsStr, jerr := json.Marshal(ids)
	if jerr != nil {
		return nil, jerr
	}

	args := ecs.DescribeInstancesArgs{
		RegionId:    client.Region,
		InstanceIds: string(idsStr),
	}

	instances, _, errs := client.ecsconn.DescribeInstances(&args)

	if errs != nil {
		return nil, errs
	}

	return instances, nil
}

func (client *AliyunClient) QueryInstancesById(id string) (instance *ecs.InstanceAttributesType, err error) {
	ids := []string{id}

	instances, errs := client.QueryInstancesByIds(ids)
	if errs != nil {
		return nil, errs
	}

	if len(instances) == 0 {
		return nil, GetNotFoundErrorFromString(InstanceNotFound)
	}

	return &instances[0], nil
}

func (client *AliyunClient) QueryInstanceSystemDisk(id string) (disk *ecs.DiskItemType, err error) {
	args := ecs.DescribeDisksArgs{
		RegionId:   client.Region,
		InstanceId: string(id),
		DiskType:   ecs.DiskTypeAllSystem,
	}
	disks, _, err := client.ecsconn.DescribeDisks(&args)
	if err != nil {
		return nil, err
	}
	if len(disks) == 0 {
		return nil, common.GetClientErrorFromString(SystemDiskNotFound)
	}

	return &disks[0], nil
}

// ResourceAvailable check resource available for zone
func (client *AliyunClient) ResourceAvailable(zone *ecs.ZoneType, resourceType ecs.ResourceType) error {
	available := false
	for _, res := range zone.AvailableResourceCreation.ResourceTypes {
		if res == resourceType {
			available = true
		}
	}
	if !available {
		return fmt.Errorf("%s is not available in %s zone of %s region", resourceType, zone.ZoneId, client.Region)
	}

	return nil
}

func (client *AliyunClient) DiskAvailable(zone *ecs.ZoneType, diskCategory ecs.DiskCategory) error {
	available := false
	for _, dist := range zone.AvailableDiskCategories.DiskCategories {
		if dist == diskCategory {
			available = true
		}
	}
	if !available {
		return fmt.Errorf("%s is not available in %s zone of %s region", diskCategory, zone.ZoneId, client.Region)
	}
	return nil
}

// todo: support syc
func (client *AliyunClient) JoinSecurityGroups(instanceId string, securityGroupIds []string) error {
	for _, sid := range securityGroupIds {
		err := client.ecsconn.JoinSecurityGroup(instanceId, sid)
		if err != nil {
			e, _ := err.(*common.Error)
			if e.ErrorResponse.Code != InvalidInstanceIdAlreadyExists {
				return err
			}
		}
	}

	return nil
}

func (client *AliyunClient) LeaveSecurityGroups(instanceId string, securityGroupIds []string) error {
	for _, sid := range securityGroupIds {
		err := client.ecsconn.LeaveSecurityGroup(instanceId, sid)
		if err != nil {
			e, _ := err.(*common.Error)
			if e.ErrorResponse.Code != InvalidSecurityGroupIdNotFound {
				return err
			}
		}
	}

	return nil
}

func (client *AliyunClient) DescribeSecurity(securityGroupId string) (*ecs.DescribeSecurityGroupAttributeResponse, error) {

	args := &ecs.DescribeSecurityGroupAttributeArgs{
		RegionId:        client.Region,
		SecurityGroupId: securityGroupId,
	}

	return client.ecsconn.DescribeSecurityGroupAttribute(args)
}

func (client *AliyunClient) DescribeSecurityByAttr(securityGroupId, direction, nicType string) (*ecs.DescribeSecurityGroupAttributeResponse, error) {

	args := &ecs.DescribeSecurityGroupAttributeArgs{
		RegionId:        client.Region,
		SecurityGroupId: securityGroupId,
		Direction:       direction,
		NicType:         ecs.NicType(nicType),
	}

	return client.ecsconn.DescribeSecurityGroupAttribute(args)
}

func (client *AliyunClient) DescribeSecurityGroupRule(securityGroupId, direction, nicType, ipProtocol, portRange string) (*ecs.PermissionType, error) {
	sg, err := client.DescribeSecurityByAttr(securityGroupId, direction, nicType)
	if err != nil {
		return nil, err
	}

	for _, p := range sg.Permissions.Permission {
		if strings.ToLower(string(p.IpProtocol)) == ipProtocol && p.PortRange == portRange {
			return &p, nil
		}
	}
	return nil, GetNotFoundErrorFromString("Security group rule not found")

}

func (client *AliyunClient) RevokeSecurityGroup(args *ecs.RevokeSecurityGroupArgs) error {
	//when the rule is not exist, api will return success(200)
	return client.ecsconn.RevokeSecurityGroup(args)
}

func (client *AliyunClient) RevokeSecurityGroupEgress(args *ecs.RevokeSecurityGroupEgressArgs) error {
	//when the rule is not exist, api will return success(200)
	return client.ecsconn.RevokeSecurityGroupEgress(args)
}

func (client *AliyunClient) CheckParameterValidity(d *schema.ResourceData, meta interface{}) (map[ResourceKeyType]interface{}, error) {
	// Before creating resources, check input parameters validity according available zone.
	// If availability zone is nil, it will return all of supported resources in the current.
	conn := meta.(*AliyunClient).ecsconn
	zones, err := conn.DescribeZones(getRegion(d, meta))
	if err != nil {
		return nil, fmt.Errorf("Error DescribeZone: %#v", err)
	}

	if zones == nil || len(zones) < 1 {
		return nil, fmt.Errorf("There are no availability zones in the region: %#v.", getRegion(d, meta))
	}

	zoneId := ""
	if val, ok := d.GetOk("availability_zone"); ok {
		zoneId = val.(string)
	}

	valid := false
	var validZones []string
	for _, zone := range zones {
		if zoneId != "" && zone.ZoneId == zoneId {
			valid = true
			zones = append(make([]ecs.ZoneType, 1), zone)
			break
		}
		validZones = append(validZones, zone.ZoneId)
	}
	if zoneId != "" && !valid {
		return nil, fmt.Errorf("Availability zone %s is not supported in the region %s. Expected availability zones: %s.",
			zoneId, getRegion(d, meta), strings.Join(validZones, ", "))
	}

	// Retrieve series instance type family
	ioOptimized := ecs.IoOptimizedOptimized
	var expectedFamilies []string
	outdatedFamiliesMap, expectedFamiliesMap, err := client.FetchSpecifiedInstanceTypeFamily(getRegion(d, meta), zoneId, []string{GenerationOne, GenerationTwo}, zones)
	if err != nil {
		return nil, err
	}
	for key := range expectedFamiliesMap {
		expectedFamilies = append(expectedFamilies, key)
	}

	var instanceType string
	if insType, ok := d.GetOk("available_instance_type"); ok {
		instanceType = insType.(string)
	} else if insType, ok := d.GetOk("instance_type"); ok {
		instanceType = insType.(string)
	}
	if instanceType != "" {

		instanceTypeSplit := strings.Split(instanceType, DOT_SEPARATED)
		prefix := string(instanceTypeSplit[0] + DOT_SEPARATED + instanceTypeSplit[1])
		var instanceTypeObject ecs.InstanceTypeItemType
		_, outdatedOk := outdatedFamiliesMap[prefix]
		_, expectedOk := expectedFamiliesMap[prefix]
		if outdatedOk || expectedOk {
			mapInstanceTypes, err := client.FetchSpecifiedInstanceTypesByFamily(zoneId, prefix, zones)
			if err != nil {
				return nil, err
			}
			if value, ok := mapInstanceTypes[instanceType]; ok {
				instanceTypeObject = value
			}
			var validInstanceTypes []string
			for key, value := range mapInstanceTypes {
				if instanceType == key {
					instanceTypeObject = mapInstanceTypes[key]
					break
				}
				validInstanceTypes = append(validInstanceTypes, fmt.Sprintf("%s(%dCPU,%.0fGB)", key, value.CpuCoreCount, value.MemorySize))
			}
			if instanceTypeObject.InstanceTypeId == "" {
				if zoneId == "" {
					return nil, fmt.Errorf("Instance type %s is not supported in the region %s. Expected instance types of family %s: %s.",
						instanceType, getRegion(d, meta), prefix, strings.Join(validInstanceTypes, ", "))
				}
				return nil, fmt.Errorf("Instance type %s is not supported in the availability zone %s. Expected instance types of family %s: %s.",
					instanceType, zoneId, prefix, strings.Join(validInstanceTypes, ", "))
			}

			outDisk := false
			if disk, ok := d.GetOk("system_disk_category"); ok {
				_, outDisk = OutdatedDiskCategory[ecs.DiskCategory(disk.(string))]
			}

			if expectedOk && outDisk {
				return nil, fmt.Errorf("Instance type %s can't support 'cloud' as instance system disk. "+
					"Please change your disk category to efficient disk '%s' or '%s'.", instanceType, ecs.DiskCategoryCloudSSD, ecs.DiskCategoryCloudEfficiency)
			}
			if outdatedOk {
				var expectedEqualCpus []string
				var expectedEqualMoreCpus []string
				for _, fam := range expectedFamilies {
					mapInstanceTypes, err := client.FetchSpecifiedInstanceTypesByFamily(zoneId, fam, zones)
					if err != nil {
						return nil, err
					}
					for _, value := range mapInstanceTypes {
						if instanceTypeObject.CpuCoreCount == value.CpuCoreCount {
							expectedEqualCpus = append(expectedEqualCpus, fmt.Sprintf("%s(%dCPU,%.0fGB)", value.InstanceTypeId, value.CpuCoreCount, value.MemorySize))
						} else if instanceTypeObject.CpuCoreCount*2 == value.CpuCoreCount {
							expectedEqualMoreCpus = append(expectedEqualMoreCpus, fmt.Sprintf("%s(%dCPU,%.0fGB)", value.InstanceTypeId, value.CpuCoreCount, value.MemorySize))
						}
					}
				}
				expectedInstanceTypes := expectedEqualMoreCpus
				if len(expectedEqualCpus) > 0 {
					expectedInstanceTypes = expectedEqualCpus
				}

				if out, ok := d.GetOk("is_outdated"); !(ok && out.(bool)) {
					return nil, fmt.Errorf("The current instance type %s(%dCPU,%.0fGB) has been outdated. Expect to use the upgraded instance types: %s. You can keep the instance type %s by setting 'is_outdated' to true.",
						instanceType, instanceTypeObject.CpuCoreCount, instanceTypeObject.MemorySize, strings.Join(expectedInstanceTypes, ", "), instanceType)
				} else {
					// Check none io optimized and cloud
					_, typeOk := NoneIoOptimizedInstanceType[instanceType]
					_, famOk := NoneIoOptimizedFamily[prefix]
					_, halfOk := HalfIoOptimizedFamily[prefix]
					if typeOk || famOk {
						if outDisk {
							ioOptimized = ecs.IoOptimizedNone
						} else {
							return nil, fmt.Errorf("The current instance type %s is no I/O optimized, and it only supports 'cloud' as instance system disk. "+
								"Suggest to upgrade instance type and use efficient disk. Expected instance types: %s.", instanceType, strings.Join(expectedInstanceTypes, ", "))
						}
					} else if outDisk {
						if halfOk {
							ioOptimized = ecs.IoOptimizedNone
						} else {
							return nil, fmt.Errorf("The current instance type %s is I/O optimized, and it can't support 'cloud' as instance system disk. "+
								"Suggest to upgrade instance type and use efficient disk. Expectd instance types: %s.", instanceType, strings.Join(expectedInstanceTypes, ", "))
						}
					}
				}
			}

		} else {
			var validFamilies []string
			for key := range expectedFamiliesMap {
				validFamilies = append(validFamilies, key)
			}

			if len(validFamilies) < 1 {
				return nil, fmt.Errorf("There is no available instance type in the current availability zone." +
					"Please change availability zone or region and try again.")
			}
			if zoneId == "" {
				return nil, fmt.Errorf("Instance type family %s is not supported in the region %s. Expected instance type families: %s.",
					prefix, getRegion(d, meta), strings.Join(validFamilies, ", "))
			}
			return nil, fmt.Errorf("Instance type family %s is not supported in the availability zone %s. Expected instance type families: %s.",
				prefix, zoneId, strings.Join(validFamilies, ", "))
		}
	}

	if instanceTypeFamily, ok := d.GetOk("instance_type_family"); ok {

		_, outdatedOk := outdatedFamiliesMap[instanceTypeFamily.(string)]
		_, expectedOk := expectedFamiliesMap[instanceTypeFamily.(string)]
		if outdatedOk || !expectedOk {
			var validFamilies []string
			for key := range expectedFamiliesMap {
				validFamilies = append(validFamilies, key)
			}
			if len(validFamilies) < 1 {
				return nil, fmt.Errorf("There is no available instance type family in the current availability zone." +
					"Please change availability zone or region and try again.")
			}
			if zoneId == "" {
				return nil, fmt.Errorf("Instance type family %s is not supported in the region %s. Expected instance type families: %s.",
					instanceTypeFamily, getRegion(d, meta), strings.Join(validFamilies, ", "))
			}
			return nil, fmt.Errorf("Instance type family %s is not supported in the availability zone %s. Expected instance type families: %s.",
				instanceTypeFamily, zoneId, strings.Join(validFamilies, ", "))
		}
	}

	validData := make(map[ResourceKeyType]interface{})
	mapZones := make(map[string]ecs.ZoneType)
	mapSupportedInstanceTypes := make(map[string]string)
	mapUpgradedInstanceTypes := make(map[string]string)
	mapOutdatedInstanceTypes := make(map[string]string)
	mapOutdatedDiskCategories := make(map[ecs.DiskCategory]ecs.DiskCategory)
	mapDiskCategories := make(map[ecs.DiskCategory]ecs.DiskCategory)
	for _, zone := range zones {
		//var validInstanceTypes []string
		for _, insType := range zone.AvailableInstanceTypes.InstanceTypes {
			insTypeSplit := strings.Split(insType, DOT_SEPARATED)
			prefix := string(insTypeSplit[0] + DOT_SEPARATED + insTypeSplit[1])
			if _, ok := outdatedFamiliesMap[prefix]; ok {
				mapOutdatedInstanceTypes[insType] = prefix
			}
			if _, ok := expectedFamiliesMap[prefix]; ok {
				mapUpgradedInstanceTypes[insType] = prefix
			}
			mapSupportedInstanceTypes[insType] = prefix
		}
		if len(zone.AvailableDiskCategories.DiskCategories) < 1 {
			continue
		}
		//var validDiskCategories []ecs.DiskCategory
		for _, category := range zone.AvailableDiskCategories.DiskCategories {
			if _, ok := SupportedDiskCategory[category]; ok {
				mapDiskCategories[category] = category
			}
			if _, ok := OutdatedDiskCategory[category]; ok {
				mapOutdatedDiskCategories[category] = category
			}
		}
		resources := zone.AvailableResources.ResourcesInfo
		if len(resources) < 1 {
			continue
		}
		mapZones[zone.ZoneId] = zone

	}

	validData[ZoneKey] = mapZones
	validData[InstanceTypeKey] = mapSupportedInstanceTypes
	validData[UpgradedInstanceTypeKey] = mapUpgradedInstanceTypes
	validData[OutdatedInstanceTypeKey] = mapOutdatedInstanceTypes
	validData[UpgradedInstanceTypeFamilyKey] = expectedFamiliesMap
	validData[OutdatedInstanceTypeFamilyKey] = outdatedFamiliesMap
	validData[OutdatedDiskCategoryKey] = mapOutdatedDiskCategories
	validData[DiskCategoryKey] = mapDiskCategories
	validData[IoOptimizedKey] = ioOptimized

	return validData, nil
}

func (client *AliyunClient) FetchSpecifiedInstanceTypeFamily(regionId common.Region, zoneId string, generations []string, all_zones []ecs.ZoneType) (map[string]ecs.InstanceTypeFamily, map[string]ecs.InstanceTypeFamily, error) {
	// Describe specified series instance type families
	outdatedFamiliesMap := make(map[string]ecs.InstanceTypeFamily)
	upgradedFamiliesMap := make(map[string]ecs.InstanceTypeFamily)
	response, err := client.ecsconn.DescribeInstanceTypeFamilies(&ecs.DescribeInstanceTypeFamiliesArgs{
		RegionId: regionId,
	})
	if err != nil {
		return nil, nil, fmt.Errorf("Error DescribeInstanceTypeFamilies: %#v.", err)
	}
	tempOutdatedMap := make(map[string]string)
	for _, gen := range generations {
		tempOutdatedMap[gen] = gen
	}

	for _, family := range response.InstanceTypeFamilies.InstanceTypeFamily {
		if _, ok := tempOutdatedMap[family.Generation]; ok {
			outdatedFamiliesMap[family.InstanceTypeFamilyId] = family
			continue
		}
		upgradedFamiliesMap[family.InstanceTypeFamilyId] = family
	}

	// Filter specified zone's instance type families, and make them fit for specified generation
	if zoneId != "" {
		outdatedValidFamilies := make(map[string]ecs.InstanceTypeFamily)
		upgradedValidFamilies := make(map[string]ecs.InstanceTypeFamily)
		for _, zone := range all_zones {
			if zone.ZoneId == zoneId {
				for _, resource := range zone.AvailableResources.ResourcesInfo {
					families := resource.InstanceTypeFamilies[ecs.SupportedInstanceTypeFamily]
					for _, familyId := range families {
						if val, ok := outdatedFamiliesMap[familyId]; ok {
							outdatedValidFamilies[familyId] = val
						}
						if val, ok := upgradedFamiliesMap[familyId]; ok {
							upgradedValidFamilies[familyId] = val
						}
					}

				}
				return outdatedValidFamilies, upgradedValidFamilies, nil
			}
		}
	}
	return outdatedFamiliesMap, upgradedFamiliesMap, nil
}

func (client *AliyunClient) FetchSpecifiedInstanceTypesByFamily(zoneId, instanceTypeFamily string, all_zones []ecs.ZoneType) (map[string]ecs.InstanceTypeItemType, error) {
	// Describe all instance types of specified families
	types, err := client.ecsconn.DescribeInstanceTypesNew(&ecs.DescribeInstanceTypesArgs{
		InstanceTypeFamily: instanceTypeFamily,
	})
	if err != nil {
		return nil, fmt.Errorf("Error DescribeInstanceTypes: %#v.", err)
	}
	instanceTypes := make(map[string]ecs.InstanceTypeItemType)
	for _, ty := range types {
		instanceTypes[ty.InstanceTypeId] = ty
	}

	// Filter specified zone's instance types, and make them fit for specified families
	if zoneId != "" {
		validInstanceTypes := make(map[string]ecs.InstanceTypeItemType)
		for _, zone := range all_zones {
			if zone.ZoneId == zoneId {
				for _, resource := range zone.AvailableResources.ResourcesInfo {
					types := resource.InstanceTypes[ecs.SupportedInstanceType]
					for _, ty := range types {
						if val, ok := instanceTypes[ty]; ok {
							validInstanceTypes[ty] = val
						}
					}

				}
				return validInstanceTypes, nil
			}
		}
	}
	return instanceTypes, nil
}

func (client *AliyunClient) QueryInstancesWithKeyPair(region common.Region, instanceIds, keypair string) ([]interface{}, []ecs.InstanceAttributesType, error) {
	var instance_ids []interface{}
	var instanceList []ecs.InstanceAttributesType

	conn := client.ecsconn
	args := &ecs.DescribeInstancesArgs{
		RegionId: region,
	}
	pagination := getPagination(1, 50)
	for true {
		if instanceIds != "" {
			args.InstanceIds = instanceIds
		}
		args.Pagination = pagination
		instances, _, err := conn.DescribeInstances(args)
		if err != nil {
			return nil, nil, fmt.Errorf("Error DescribeInstances: %#v", err)
		}
		for _, inst := range instances {
			if inst.KeyPairName == keypair {
				instance_ids = append(instance_ids, inst.InstanceId)
				instanceList = append(instanceList, inst)
			}
		}
		if len(instances) < pagination.PageSize {
			break
		}
		pagination.PageNumber += 1
	}
	return instance_ids, instanceList, nil
}
