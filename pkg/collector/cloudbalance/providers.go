package cloudbalance

import (
	"errors"
	"fmt"
	"strings"

	bssclient "github.com/alibabacloud-go/bssopenapi-20171214/client"
	openapiclient "github.com/alibabacloud-go/darabonba-openapi/client"
	"github.com/alibabacloud-go/tea/tea"
	billing2 "github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/billing/v20180709"
	"github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/common"
	tencentErr "github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/common/errors"
	"github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/common/profile"
	"github.com/volcengine/volcengine-go-sdk/service/billing"
	"github.com/volcengine/volcengine-go-sdk/volcengine"
	"github.com/volcengine/volcengine-go-sdk/volcengine/credentials"
	"github.com/volcengine/volcengine-go-sdk/volcengine/session"
)

// QueryBalance queries balance based on provider
func QueryBalance(account AccountConfig) (float64, error) {
	var balanceStr string
	var err error

	switch account.Provider {
	case AliCloud:
		balanceStr, err = queryAlibabaCloudBalance(
			account.AccessKeyID,
			account.AccessKeySecret,
			account.RegionID,
		)
	case VolcEngine:
		balanceStr, err = queryVolcEngineBalance(
			account.AccessKeyID,
			account.AccessKeySecret,
			account.RegionID,
		)
	case TencentCloud:
		balanceStr, err = queryTencentCloudBalance(
			account.AccessKeyID,
			account.AccessKeySecret,
			account.RegionID,
		)
	default:
		return 0, fmt.Errorf("unsupported provider: %s", account.Provider)
	}

	if err != nil {
		return 0, err
	}

	return parseBalance(balanceStr)
}

// queryAlibabaCloudBalance queries Alibaba Cloud balance
func queryAlibabaCloudBalance(accessKeyId, accessKeySecret, regionId string) (string, error) {
	config := &openapiclient.Config{
		AccessKeyId:     tea.String(accessKeyId),
		AccessKeySecret: tea.String(accessKeySecret),
		RegionId:        tea.String(regionId),
		Endpoint:        tea.String("business.aliyuncs.com"),
	}

	bssClient, err := bssclient.NewClient(config)
	if err != nil {
		return "", fmt.Errorf("failed to create client: %w", err)
	}

	response, err := bssClient.QueryAccountBalance()
	if err != nil {
		return "", fmt.Errorf("failed to query balance: %w", err)
	}

	if !tea.BoolValue(response.Body.Success) {
		return "", fmt.Errorf("query failed, Code: %s, Message: %s, RequestId: %s",
			tea.StringValue(response.Body.Code),
			tea.StringValue(response.Body.Message),
			tea.StringValue(response.Body.RequestId))
	}

	if response.Body.Data == nil || response.Body.Data.AvailableAmount == nil {
		return "", fmt.Errorf("no balance data in response")
	}

	return tea.StringValue(response.Body.Data.AvailableAmount), nil
}

// queryVolcEngineBalance queries VolcEngine balance
func queryVolcEngineBalance(accessKeyId, accessKeySecret, regionId string) (string, error) {
	config := volcengine.NewConfig()
	if regionId != "" {
		config = config.WithRegion(regionId)
	}
	config = config.WithCredentials(credentials.NewStaticCredentials(accessKeyId, accessKeySecret, ""))

	sess, err := session.NewSession(config)
	if err != nil {
		return "", fmt.Errorf("failed to create session: %w", err)
	}

	svc := billing.New(sess)
	queryBalanceAcctInput := &billing.QueryBalanceAcctInput{}
	response, err := svc.QueryBalanceAcct(queryBalanceAcctInput)
	if err != nil {
		return "", fmt.Errorf("failed to query balance: %w", err)
	}

	if response.AvailableBalance == nil {
		return "", fmt.Errorf("no balance data in response")
	}

	return fmt.Sprintf("%s", *response.AvailableBalance), nil
}

// queryTencentCloudBalance queries Tencent Cloud balance
func queryTencentCloudBalance(secretId, secretKey, regionId string) (string, error) {
	credential := common.NewCredential(secretId, secretKey)
	cpf := profile.NewClientProfile()
	cpf.HttpProfile.Endpoint = "billing.tencentcloudapi.com"

	client, err := billing2.NewClient(credential, regionId, cpf)
	if err != nil {
		return "", fmt.Errorf("failed to create client: %w", err)
	}

	request := billing2.NewDescribeAccountBalanceRequest()
	response, err := client.DescribeAccountBalance(request)
	var tencentCloudSDKError *tencentErr.TencentCloudSDKError
	if errors.As(err, &tencentCloudSDKError) {
		return "", fmt.Errorf("API error: %w", err)
	}
	if err != nil {
		return "", fmt.Errorf("failed to query balance: %w", err)
	}

	if response.Response == nil || response.Response.RealBalance == nil {
		return "", fmt.Errorf("no balance data in response")
	}

	balanceYuan := float64(*response.Response.RealBalance) / 100
	return fmt.Sprintf("%.2f", balanceYuan), nil
}

// parseBalance converts balance string to float64
func parseBalance(balance string) (float64, error) {
	// Remove commas from the balance string
	cleanBalance := strings.ReplaceAll(balance, ",", "")
	var balanceFloat float64
	_, err := fmt.Sscanf(cleanBalance, "%f", &balanceFloat)
	return balanceFloat, err
}
