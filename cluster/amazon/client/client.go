//go:generate mockery -name=Client

package client

import (
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ec2"
)

// A Client to an Amazon EC2 region.
type Client interface {
	DescribeInstances([]*ec2.Filter) (*ec2.DescribeInstancesOutput, error)
	RunInstances(*ec2.RunInstancesInput) (*ec2.Reservation, error)
	TerminateInstances(ids []string) error

	DescribeSpotInstanceRequests(ids []string, filters []*ec2.Filter) (
		[]*ec2.SpotInstanceRequest, error)
	RequestSpotInstances(spotPrice string, count int64,
		launchSpec *ec2.RequestSpotLaunchSpecification) (
		[]*ec2.SpotInstanceRequest, error)
	CancelSpotInstanceRequests(ids []string) error

	DescribeSecurityGroup(name string) ([]*ec2.SecurityGroup, error)
	CreateSecurityGroup(name, description string) (string, error)
	AuthorizeSecurityGroup(name, src string, ranges []*ec2.IpPermission) error
	RevokeSecurityGroup(name string, ranges []*ec2.IpPermission) error
	DescribeAddresses() ([]*ec2.Address, error)
	AssociateAddress(id, allocationID string) error
	DisassociateAddress(associationID string) error

	DescribeVolumes(id string) ([]*ec2.Volume, error)
}

type awsClient struct {
	client *ec2.EC2
}

func (ac awsClient) DescribeInstances(filters []*ec2.Filter) (
	*ec2.DescribeInstancesOutput, error) {
	return ac.client.DescribeInstances(&ec2.DescribeInstancesInput{Filters: filters})
}

func (ac awsClient) RunInstances(in *ec2.RunInstancesInput) (*ec2.Reservation, error) {
	return ac.client.RunInstances(in)
}

func (ac awsClient) TerminateInstances(ids []string) error {
	_, err := ac.client.TerminateInstances(&ec2.TerminateInstancesInput{
		InstanceIds: stringSlice(ids)})
	return err
}

func (ac awsClient) DescribeSpotInstanceRequests(ids []string, filters []*ec2.Filter) (
	[]*ec2.SpotInstanceRequest, error) {
	resp, err := ac.client.DescribeSpotInstanceRequests(
		&ec2.DescribeSpotInstanceRequestsInput{
			SpotInstanceRequestIds: stringSlice(ids),
			Filters:                filters})
	return resp.SpotInstanceRequests, err
}

func (ac awsClient) RequestSpotInstances(spotPrice string, count int64,
	launchSpec *ec2.RequestSpotLaunchSpecification) (
	[]*ec2.SpotInstanceRequest, error) {

	resp, err := ac.client.RequestSpotInstances(&ec2.RequestSpotInstancesInput{
		SpotPrice:           &spotPrice,
		InstanceCount:       &count,
		LaunchSpecification: launchSpec})
	if err != nil {
		return nil, err
	}
	return resp.SpotInstanceRequests, err
}
func (ac awsClient) CancelSpotInstanceRequests(ids []string) error {
	_, err := ac.client.CancelSpotInstanceRequests(
		&ec2.CancelSpotInstanceRequestsInput{
			SpotInstanceRequestIds: stringSlice(ids)})
	return err
}

func (ac awsClient) DescribeSecurityGroup(name string) ([]*ec2.SecurityGroup, error) {
	resp, err := ac.client.DescribeSecurityGroups(&ec2.DescribeSecurityGroupsInput{
		Filters: []*ec2.Filter{{
			Name:   aws.String("group-name"),
			Values: []*string{&name}}}})
	if err != nil {
		return nil, err
	}
	return resp.SecurityGroups, err
}

func (ac awsClient) CreateSecurityGroup(name, description string) (string, error) {
	csgResp, err := ac.client.CreateSecurityGroup(&ec2.CreateSecurityGroupInput{
		GroupName:   &name,
		Description: &description})
	if err != nil {
		return "", err
	}
	return *csgResp.GroupId, err
}

func (ac awsClient) AuthorizeSecurityGroup(name, src string,
	ranges []*ec2.IpPermission) error {

	var srcPtr *string
	if src != "" {
		srcPtr = &src
	}

	_, err := ac.client.AuthorizeSecurityGroupIngress(
		&ec2.AuthorizeSecurityGroupIngressInput{
			GroupName:               &name,
			SourceSecurityGroupName: srcPtr,
			IpPermissions:           ranges})
	return err
}

func (ac awsClient) RevokeSecurityGroup(name string, ranges []*ec2.IpPermission) error {
	_, err := ac.client.RevokeSecurityGroupIngress(
		&ec2.RevokeSecurityGroupIngressInput{
			GroupName:     &name,
			IpPermissions: ranges})
	return err
}

func (ac awsClient) DescribeAddresses() ([]*ec2.Address, error) {
	resp, err := ac.client.DescribeAddresses(nil)
	if err != nil {
		return nil, err
	}
	return resp.Addresses, err
}

func (ac awsClient) AssociateAddress(id, allocationID string) error {
	_, err := ac.client.AssociateAddress(&ec2.AssociateAddressInput{
		InstanceId:   &id,
		AllocationId: &allocationID})
	return err
}

func (ac awsClient) DisassociateAddress(associationID string) error {
	_, err := ac.client.DisassociateAddress(&ec2.DisassociateAddressInput{
		AssociationId: &associationID})
	return err
}

func (ac awsClient) DescribeVolumes(id string) ([]*ec2.Volume, error) {
	resp, err := ac.client.DescribeVolumes(&ec2.DescribeVolumesInput{
		Filters: []*ec2.Filter{{
			Name:   aws.String("volume-id"),
			Values: []*string{&id}}}})
	if err != nil {
		return nil, err
	}
	return resp.Volumes, err
}

// New creates a new Client.
func New(region string) Client {
	session := session.New()
	session.Config.Region = &region
	return awsClient{ec2.New(session)}
}

// The amazon API makes a distinction between `nil` which means "this parameter was
// omitted" and `[]*string` which means "this parameter was provided with no elements".
// aws.StringSlice() clobbers that distinction, so we wrap with stringSlice.
func stringSlice(slice []string) []*string {
	if slice == nil {
		return nil
	}
	return aws.StringSlice(slice)
}
