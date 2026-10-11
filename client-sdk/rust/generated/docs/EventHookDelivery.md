# EventHookDelivery

## Properties

Name | Type | Description | Notes
------------ | ------------- | ------------- | -------------
**attempt_count** | **i32** |  | 
**created_at** | **chrono::DateTime<chrono::FixedOffset>** |  | 
**delivered_at** | Option<**chrono::DateTime<chrono::FixedOffset>**> |  | [optional]
**duration_ms** | **i64** |  | 
**error_message** | Option<**String**> |  | [optional]
**event_hook_id** | Option<**serde_json::Value**> |  | 
**event_id** | Option<**serde_json::Value**> |  | 
**event_type** | **String** |  | 
**http_response_status** | Option<**i32**> |  | [optional]
**id** | Option<**serde_json::Value**> |  | 
**is_delivered** | **bool** |  | 
**payload** | Option<**serde_json::Value**> |  | 
**result** | Option<**String**> |  | [optional]

[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)


