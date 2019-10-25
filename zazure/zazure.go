package zazure

import (
	"context"
	"fmt"

	//	"github.com/Azure-Samples/azure-sdk-for-go-samples/internal/config"
	//"github.com/Azure-Samples/azure-sdk-for-go-samples/internal/iam"

	//	eventhub "github.com/Azure/azure-event-hubs-go"
	eventhubs "github.com/Azure/azure-event-hubs-go"
	"github.com/Azure/azure-sdk-for-go/profiles/latest/eventhub/mgmt/eventhub"
	"github.com/Azure/go-autorest/autorest/to"
	"github.com/torlangballe/zutil/zlog"
)

// eventhubs "github.com/Azure/azure-event-hubs-go"

func getNamespacesClient() eventhub.NamespacesClient {
	nsClient := eventhub.NewNamespacesClient(ConfigSubscriptionID)
	auth, _ := getResourceManagementAuthorizer()
	nsClient.Authorizer = auth
	nsClient.AddToUserAgent(ConfigUserAgent)
	return nsClient
}

func CreateNamespace(ctx context.Context, nsName, groupName string) (*eventhub.EHNamespace, error) {
	nsClient := getNamespacesClient()
	future, err := nsClient.CreateOrUpdate(
		ctx,
		groupName,
		nsName,
		eventhub.EHNamespace{
			Location: to.StringPtr(ConfigLocation),
		},
	)
	fmt.Println("Create NS:", err)
	if err != nil {
		return nil, err
	}

	err = future.WaitForCompletionRef(ctx, nsClient.Client)
	if err != nil {
		return nil, err
	}

	result, err := future.Result(nsClient)
	return &result, err
}

func SendEvent(ctx context.Context, connectionStr, message string) error {
	hub, err := eventhubs.NewHubFromConnectionString(connectionStr)
	if err != nil {
		return zlog.Error(err, "NewHub", connectionStr)
	}
	err = hub.Send(ctx, eventhubs.NewEventFromString(message))
	if err != nil {
		return zlog.Error(err, "Send", connectionStr)
	}
	return nil
}
