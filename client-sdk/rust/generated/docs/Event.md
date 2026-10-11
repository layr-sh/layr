# Event

## Properties

Name | Type | Description | Notes
------------ | ------------- | ------------- | -------------
**action** | **String** |  | 
**actor** | [**models::EventActor**](EventActor.md) |  | 
**context** | [**models::EventContext**](EventContext.md) |  | 
**created_at** | **chrono::DateTime<chrono::FixedOffset>** |  | 
**data** | Option<**std::collections::HashMap<String, serde_json::Value>**> |  | 
**id** | Option<**serde_json::Value**> |  | 
**metadata** | Option<**std::collections::HashMap<String, serde_json::Value>**> |  | 
**resource_id** | Option<**String**> |  | [optional]
**resource_type** | **String** |  | 
**r#type** | **String** |  | 

[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)


