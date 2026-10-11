
# ServiceAccount

ServiceAccount schema

## Properties

Name | Type
------------ | -------------
`allowedIps` | Array&lt;string&gt;
`consoleUserId` | string
`createdAt` | Date
`description` | string
`expiresAt` | Date
`id` | string
`isEnabled` | boolean
`keyPrefix` | string
`lastUpdatedAt` | Date
`lastUsedAt` | Date
`name` | string
`scopes` | Array&lt;string&gt;

## Example

```typescript
import type { ServiceAccount } from ''

// TODO: Update the object below with actual values
const example = {
  "allowedIps": null,
  "consoleUserId": null,
  "createdAt": null,
  "description": null,
  "expiresAt": null,
  "id": null,
  "isEnabled": null,
  "keyPrefix": null,
  "lastUpdatedAt": null,
  "lastUsedAt": null,
  "name": null,
  "scopes": null,
} satisfies ServiceAccount

console.log(example)

// Convert the instance to a JSON string
const exampleJSON: string = JSON.stringify(example)
console.log(exampleJSON)

// Parse the JSON string back to an object
const exampleParsed = JSON.parse(exampleJSON) as ServiceAccount
console.log(exampleParsed)
```

[[Back to top]](#) [[Back to API list]](../README.md#api-endpoints) [[Back to Model list]](../README.md#models) [[Back to README]](../README.md)


