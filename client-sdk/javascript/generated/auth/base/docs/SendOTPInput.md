
# SendOTPInput

SendOTPInput schema

## Properties

Name | Type
------------ | -------------
`captchaToken` | string
`purpose` | string
`recipient` | string

## Example

```typescript
import type { SendOTPInput } from ''

// TODO: Update the object below with actual values
const example = {
  "captchaToken": null,
  "purpose": null,
  "recipient": null,
} satisfies SendOTPInput

console.log(example)

// Convert the instance to a JSON string
const exampleJSON: string = JSON.stringify(example)
console.log(exampleJSON)

// Parse the JSON string back to an object
const exampleParsed = JSON.parse(exampleJSON) as SendOTPInput
console.log(exampleParsed)
```

[[Back to top]](#) [[Back to API list]](../README.md#api-endpoints) [[Back to Model list]](../README.md#models) [[Back to README]](../README.md)


