package cash_entries

import (
	"context"
	"fmt"

	firebase "firebase.google.com/go"
	"github.com/GoogleCloudPlatform/functions-framework-go/functions"
	"github.com/cloudevents/sdk-go/v2/event"
	"github.com/deaglefrenzy/trigger-test/models"
	"github.com/googleapis/google-cloudevents-go/cloud/firestoredata"
	"google.golang.org/protobuf/proto"
)

func init() {
	functions.CloudEvent("detectCashEntriesChanges", DetectCashEntriesChanges)
}

func DetectCashEntriesChanges(ctx context.Context, event event.Event) error {
	var data firestoredata.DocumentEventData
	options := proto.UnmarshalOptions{
		DiscardUnknown: true,
	}
	err := options.Unmarshal(event.Data(), &data)

	if err != nil {
		return fmt.Errorf("proto.Unmarshal: %w", err)
	}

	if arrValue := data.Value.Fields["cash_entries"].GetArrayValue(); arrValue != nil {
		arr := arrValue.Values
		var oldArr []*firestoredata.Value
		if oldV, ok := data.OldValue.Fields["cash_entries"]; ok {
			if oldArrValue := oldV.GetArrayValue(); oldArrValue != nil {
				oldArr = oldArrValue.Values
			}
		}

		for _, entry := range arr {
			found := false
			for _, oldEntry := range oldArr {
				if proto.Equal(entry, oldEntry) {
					found = true
					break
				}
			}
			if !found {
				var PendingEntries models.PendingEntries
				var cashEntry models.CashEntry

				if mapValue := entry.GetMapValue(); mapValue != nil {
					fields := mapValue.Fields

					cashEntry.CreatedAt = fields["created_at"].GetTimestampValue().AsTime()
					cashEntry.Description = fields["description"].GetStringValue()
					cashEntry.Expense = fields["expense"].GetBooleanValue()
					cashEntry.Username = fields["username"].GetStringValue()
					cashEntry.UUID = fields["uuid"].GetStringValue()
					//cashEntry.Value = fields["value"].GetIntegerValue()
					if val, ok := fields["value"]; ok {
						if intVal := val.GetIntegerValue(); intVal != 0 || (val.ValueType == &firestoredata.Value_IntegerValue{}) {
							cashEntry.Value = float64(intVal)
						} else if doubleVal := val.GetDoubleValue(); doubleVal != 0 || (val.ValueType == &firestoredata.Value_DoubleValue{}) {
							cashEntry.Value = doubleVal
						} else {
							cashEntry.Value = 0
						}
					}
				}
				PendingEntries.BranchUUID = data.Value.Fields["branch_uuid"].GetStringValue()
				PendingEntries.Resolved = false
				PendingEntries.ResolvedAt = nil
				PendingEntries.ResolvedBy = nil
				PendingEntries.Notes = nil
				PendingEntries.ShiftData.UUID = data.Value.Fields["uuid"].GetStringValue()
				PendingEntries.ShiftData.StartTime = data.Value.Fields["created_at"].GetTimestampValue().AsTime()
				PendingEntries.ShiftData.MainShiftUser = data.Value.Fields["username"].GetStringValue()
				PendingEntries.CashEntry = cashEntry

				//log.Printf("Pending expense entry inserted: %+v", PendingEntries)

				app, err := firebase.NewApp(ctx, nil)
				if err != nil {
					return fmt.Errorf("fail to connect: %w", err)
				}

				fs, err := app.Firestore(ctx)
				if err != nil {
					return fmt.Errorf("fail to connect: %w", err)
				}

				ref := fs.Collection("pending_expense_entries").NewDoc()
				if _, err := ref.Set(ctx, PendingEntries); err != nil {
					return fmt.Errorf("failed to create pending expense entries: %w", err)
				}
			}
		}
	}

	return nil
}
