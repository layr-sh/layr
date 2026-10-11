
# EventHook

EventHook schema

## Properties

Name | Type
------------ | -------------
`createdAt` | Date
`driver` | string
`eventTypes` | Array&lt;string&gt;
`httpTargetUrl` | string
`id` | any
`isEnabled` | boolean
`lastUpdatedAt` | Date
`maxRetries` | number
`name` | string
`signingSecretConfigured` | boolean
`sqlFunctionName` | string
`timeoutSeconds` | number

## Example

```typescript
import type { EventHook } from ''

// TODO: Update the object below with actual values
const example = {
  "createdAt": null,
  "driver": null,
  "eventTypes": null,
  "httpTargetUrl": null,
  "id": null,
  "isEnabled": null,
  "lastUpdatedAt": null,
  "maxRetries": null,
  "name": null,
  "signingSecretConfigured": null,
  "sqlFunctionName": null,
  "timeoutSeconds": null,
} satisfies EventHook

console.log(example)

// Convert the instance to a JSON string
const exampleJSON: string = JSON.stringify(example)
console.log(exampleJSON)

// Parse the JSON string back to an object
const exampleParsed = JSON.parse(exampleJSON) as EventHook
console.log(exampleParsed)
```

[[Back to top]](#) [[Back to API list]](../README.md#api-endpoints) [[Back to Model list]](../README.md#models) [[Back to README]](../README.md)


