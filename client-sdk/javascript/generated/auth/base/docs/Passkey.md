
# Passkey

Passkey schema

## Properties

Name | Type
------------ | -------------
`createdAt` | Date
`friendlyName` | string
`id` | string
`lastUsedAt` | Date
`transports` | Array&lt;string&gt;

## Example

```typescript
import type { Passkey } from ''

// TODO: Update the object below with actual values
const example = {
  "createdAt": null,
  "friendlyName": null,
  "id": null,
  "lastUsedAt": null,
  "transports": null,
} satisfies Passkey

console.log(example)

// Convert the instance to a JSON string
const exampleJSON: string = JSON.stringify(example)
console.log(exampleJSON)

// Parse the JSON string back to an object
const exampleParsed = JSON.parse(exampleJSON) as Passkey
console.log(exampleParsed)
```

[[Back to top]](#) [[Back to API list]](../README.md#api-endpoints) [[Back to Model list]](../README.md#models) [[Back to README]](../README.md)


