
# SignUpOptions

SignUpOptions schema

## Properties

Name | Type
------------ | -------------
`challenge` | string
`rpId` | string
`rpName` | string
`userId` | string
`userName` | string

## Example

```typescript
import type { SignUpOptions } from ''

// TODO: Update the object below with actual values
const example = {
  "challenge": null,
  "rpId": null,
  "rpName": null,
  "userId": null,
  "userName": null,
} satisfies SignUpOptions

console.log(example)

// Convert the instance to a JSON string
const exampleJSON: string = JSON.stringify(example)
console.log(exampleJSON)

// Parse the JSON string back to an object
const exampleParsed = JSON.parse(exampleJSON) as SignUpOptions
console.log(exampleParsed)
```

[[Back to top]](#) [[Back to API list]](../README.md#api-endpoints) [[Back to Model list]](../README.md#models) [[Back to README]](../README.md)


