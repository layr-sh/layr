# StatsResponse

## Properties

Name | Type | Description | Notes
------------ | ------------- | ------------- | -------------
**active_session_users** | **i64** |  | 
**active_sessions** | **i64** |  | 
**anonymous_users** | **i64** |  | 
**email_unverified_users** | **i64** |  | 
**email_verified_users** | **i64** |  | 
**expired_sessions** | **i64** |  | 
**federated_users** | **i64** |  | 
**generated_at** | **chrono::DateTime<chrono::FixedOffset>** |  | 
**idle_sessions** | **i64** |  | 
**locked_sessions** | **i64** |  | 
**locked_users** | **i64** |  | 
**mfa_users** | **i64** |  | 
**new_users_last_24_hours** | **i64** |  | 
**new_users_last_7_days** | **i64** |  | 
**new_users_previous_24_hours** | **i64** |  | 
**passkey_users** | **i64** |  | 
**password_users** | **i64** |  | 
**phone_unverified_users** | **i64** |  | 
**phone_verified_users** | **i64** |  | 
**providers** | Option<[**Vec<models::ProviderStats>**](ProviderStats.md)> |  | 
**registered_users** | **i64** |  | 
**registrations** | Option<[**Vec<models::RegistrationStats>**](RegistrationStats.md)> |  | 
**roles** | Option<[**Vec<models::RoleStats>**](RoleStats.md)> |  | 
**total_identities** | **i64** |  | 
**total_passkeys** | **i64** |  | 
**total_sessions** | **i64** |  | 
**total_users** | **i64** |  | 
**users_without_credentials** | **i64** |  | 

[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)


