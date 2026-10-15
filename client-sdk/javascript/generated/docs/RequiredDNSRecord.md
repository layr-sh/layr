
# RequiredDNSRecord


## Properties

Name | Type
------------ | -------------
`comment` | string
`name` | string
`required` | boolean
`status` | string
`target` | string
`type` | string

## Example

```typescript
import type { RequiredDNSRecord } from ''

// TODO: Update the object below with actual values
const example = {
  "comment": null,
  "name": null,
  "required": null,
  "status": null,
  "target": null,
  "type": null,
} satisfies RequiredDNSRecord

console.log(example)

// Convert the instance to a JSON string
const exampleJSON: string = JSON.stringify(example)
console.log(exampleJSON)

// Parse the JSON string back to an object
const exampleParsed = JSON.parse(exampleJSON) as RequiredDNSRecord
console.log(exampleParsed)
```

[[Back to top]](#) [[Back to API list]](../README.md#api-endpoints) [[Back to Model list]](../README.md#models) [[Back to README]](../README.md)


