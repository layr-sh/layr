
# SignInInput

SignInInput schema

## Properties

Name | Type
------------ | -------------
`captchaToken` | string
`email` | string
`password` | string
`phone` | string

## Example

```typescript
import type { SignInInput } from ''

// TODO: Update the object below with actual values
const example = {
  "captchaToken": null,
  "email": null,
  "password": null,
  "phone": null,
} satisfies SignInInput

console.log(example)

// Convert the instance to a JSON string
const exampleJSON: string = JSON.stringify(example)
console.log(exampleJSON)

// Parse the JSON string back to an object
const exampleParsed = JSON.parse(exampleJSON) as SignInInput
console.log(exampleParsed)
```

[[Back to top]](#) [[Back to API list]](../README.md#api-endpoints) [[Back to Model list]](../README.md#models) [[Back to README]](../README.md)


