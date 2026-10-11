# GetJobResponse

## Properties

Name | Type | Description | Notes
------------ | ------------- | ------------- | -------------
**created_at** | **chrono::DateTime<chrono::FixedOffset>** |  | 
**cron_expression** | **String** |  | 
**id** | Option<**serde_json::Value**> |  | 
**is_enabled** | **bool** |  | 
**last_run_at** | Option<**chrono::DateTime<chrono::FixedOffset>**> |  | [optional]
**last_updated_at** | **chrono::DateTime<chrono::FixedOffset>** |  | 
**name** | **String** |  | 
**next_run_at** | **chrono::DateTime<chrono::FixedOffset>** |  | 
**next_run_countdown_seconds** | **i64** |  | 
**target_payload** | **serde_json::Value** |  | 
**target_type** | **String** |  | 
**timezone** | **String** |  | 

[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)


