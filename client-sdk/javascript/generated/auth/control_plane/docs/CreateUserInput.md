
# CreateUserInput

CreateUserInput schema

## Properties

Name | Type
------------ | -------------
`email` | string
`emailVerified` | boolean
`password` | string
`phone` | string
`phoneVerified` | boolean
`properties` | { [key: string]: any | null; }
`role` | string

## Example

```typescript
import type { CreateUserInput } from ''

// TODO: Update the object below with actual values
const example = {
  "email": null,
  "emailVerified": null,
  "password": null,
  "phone": null,
  "phoneVerified": null,
  "properties": null,
  "role": null,
} satisfies CreateUserInput

console.log(example)

// Convert the instance to a JSON string
const exampleJSON: string = JSON.stringify(example)
console.log(exampleJSON)

// Parse the JSON string back to an object
const exampleParsed = JSON.parse(exampleJSON) as CreateUserInput
console.log(exampleParsed)
```

[[Back to top]](#) [[Back to API list]](../README.md#api-endpoints) [[Back to Model list]](../README.md#models) [[Back to README]](../README.md)


