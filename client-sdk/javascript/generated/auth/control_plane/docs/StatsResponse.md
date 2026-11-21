
# StatsResponse

StatsResponse schema

## Properties

Name | Type
------------ | -------------
`activeSessionUsers` | number
`activeSessions` | number
`anonymousUsers` | number
`emailUnverifiedUsers` | number
`emailVerifiedUsers` | number
`expiredSessions` | number
`federatedUsers` | number
`generatedAt` | Date
`idleSessions` | number
`lockedSessions` | number
`lockedUsers` | number
`mfaUsers` | number
`newUsersLast24Hours` | number
`newUsersLast7Days` | number
`newUsersPrevious24Hours` | number
`passkeyUsers` | number
`passwordUsers` | number
`phoneUnverifiedUsers` | number
`phoneVerifiedUsers` | number
`providers` | [Array&lt;ProviderStats&gt;](ProviderStats.md)
`registeredUsers` | number
`registrations` | [Array&lt;RegistrationStats&gt;](RegistrationStats.md)
`roles` | [Array&lt;RoleStats&gt;](RoleStats.md)
`totalIdentities` | number
`totalPasskeys` | number
`totalSessions` | number
`totalUsers` | number
`usersWithoutCredentials` | number

## Example

```typescript
import type { StatsResponse } from ''

// TODO: Update the object below with actual values
const example = {
  "activeSessionUsers": null,
  "activeSessions": null,
  "anonymousUsers": null,
  "emailUnverifiedUsers": null,
  "emailVerifiedUsers": null,
  "expiredSessions": null,
  "federatedUsers": null,
  "generatedAt": null,
  "idleSessions": null,
  "lockedSessions": null,
  "lockedUsers": null,
  "mfaUsers": null,
  "newUsersLast24Hours": null,
  "newUsersLast7Days": null,
  "newUsersPrevious24Hours": null,
  "passkeyUsers": null,
  "passwordUsers": null,
  "phoneUnverifiedUsers": null,
  "phoneVerifiedUsers": null,
  "providers": null,
  "registeredUsers": null,
  "registrations": null,
  "roles": null,
  "totalIdentities": null,
  "totalPasskeys": null,
  "totalSessions": null,
  "totalUsers": null,
  "usersWithoutCredentials": null,
} satisfies StatsResponse

console.log(example)

// Convert the instance to a JSON string
const exampleJSON: string = JSON.stringify(example)
console.log(exampleJSON)

// Parse the JSON string back to an object
const exampleParsed = JSON.parse(exampleJSON) as StatsResponse
console.log(exampleParsed)
```

[[Back to top]](#) [[Back to API list]](../README.md#api-endpoints) [[Back to Model list]](../README.md#models) [[Back to README]](../README.md)


