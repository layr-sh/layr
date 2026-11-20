# GetStatsResponse

## Properties

Name | Type | Description | Notes
------------ | ------------- | ------------- | -------------
**average_duration_ms** | Option<**f64**> |  | [optional]
**bundle_size_bytes** | **i64** |  | 
**client_errors_last_24_hours** | **i64** |  | 
**deployed_endpoints** | **i32** |  | 
**domain_routes** | **i64** |  | 
**domains_needing_tls** | **i64** |  | 
**endpoints** | Option<[**Vec<models::EndpointUsage>**](EndpointUsage.md)> |  | 
**executions_last_24_hours** | **i64** |  | 
**failed_executions_last_24_hours** | **i64** |  | 
**generated_at** | **chrono::DateTime<chrono::FixedOffset>** |  | 
**last_executed_at** | Option<**chrono::DateTime<chrono::FixedOffset>**> |  | [optional]
**p95_duration_ms** | Option<**f64**> |  | [optional]
**private_endpoints** | **i32** |  | 
**public_endpoints** | **i32** |  | 
**runtimes** | Option<[**std::collections::HashMap<String, models::RunnerHealth>**](RunnerHealth.md)> |  | 
**server_errors_last_24_hours** | **i64** |  | 
**total_deployments** | **i64** |  | 
**total_domains** | **i64** |  | 
**total_endpoints** | **i32** |  | 
**total_executions** | **i64** |  | 
**window_started_at** | **chrono::DateTime<chrono::FixedOffset>** |  | 

[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)


