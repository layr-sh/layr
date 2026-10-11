
# UpdateServiceAccountInput

UpdateServiceAccountInput schema

## Properties

Name | Type
------------ | -------------
`allowedIps` | Array&lt;string&gt;
`description` | string
`expiresAt` | Date
`isEnabled` | boolean
`name` | string
`scopes` | Array&lt;string&gt;

## Example

```typescript
import type { UpdateServiceAccountInput } from ''

// TODO: Update the object below with actual values
const example = {
  "allowedIps": null,
  "description": null,
  "expiresAt": null,
  "isEnabled": null,
  "name": null,
  "scopes": null,
} satisfies UpdateServiceAccountInput

console.log(example)

// Convert the instance to a JSON string
const exampleJSON: string = JSON.stringify(example)
console.log(exampleJSON)

// Parse the JSON string back to an object
const exampleParsed = JSON.parse(exampleJSON) as UpdateServiceAccountInput
console.log(exampleParsed)
```

[[Back to top]](#) [[Back to API list]](../README.md#api-endpoints) [[Back to Model list]](../README.md#models) [[Back to README]](../README.md)


