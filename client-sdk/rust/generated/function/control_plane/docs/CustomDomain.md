# CustomDomain

## Properties

Name | Type | Description | Notes
------------ | ------------- | ------------- | -------------
**created_at** | **chrono::DateTime<chrono::FixedOffset>** |  | 
**domain** | **String** |  | 
**id** | Option<**serde_json::Value**> |  | 
**last_updated_at** | **chrono::DateTime<chrono::FixedOffset>** |  | 
**required_dns_records** | Option<[**Vec<models::RequiredDnsRecord>**](RequiredDNSRecord.md)> |  | [optional]
**status** | **String** |  | 
**tls_acme_email** | Option<**String**> |  | [optional]
**tls_enabled** | **bool** |  | 
**tls_error_message** | Option<**String**> |  | [optional]
**tls_expires_at** | Option<**chrono::DateTime<chrono::FixedOffset>**> |  | [optional]
**tls_issued_at** | Option<**chrono::DateTime<chrono::FixedOffset>**> |  | [optional]
**tls_status** | **String** |  | 

[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)


