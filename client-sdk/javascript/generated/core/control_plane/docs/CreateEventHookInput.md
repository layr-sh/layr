
# CreateEventHookInput

CreateEventHookInput schema

## Properties

Name | Type
------------ | -------------
`driver` | string
`eventTypes` | Array&lt;string&gt;
`httpTargetUrl` | string
`isEnabled` | boolean
`maxRetries` | number
`name` | string
`signingSecret` | string
`sqlFunctionName` | string
`timeoutSeconds` | number

## Example

```typescript
import type { CreateEventHookInput } from ''

// TODO: Update the object below with actual values
const example = {
  "driver": null,
  "eventTypes": null,
  "httpTargetUrl": null,
  "isEnabled": null,
  "maxRetries": null,
  "name": null,
  "signingSecret": null,
  "sqlFunctionName": null,
  "timeoutSeconds": null,
} satisfies CreateEventHookInput

console.log(example)

// Convert the instance to a JSON string
const exampleJSON: string = JSON.stringify(example)
console.log(exampleJSON)

// Parse the JSON string back to an object
const exampleParsed = JSON.parse(exampleJSON) as CreateEventHookInput
console.log(exampleParsed)
```

[[Back to top]](#) [[Back to API list]](../README.md#api-endpoints) [[Back to Model list]](../README.md#models) [[Back to README]](../README.md)


