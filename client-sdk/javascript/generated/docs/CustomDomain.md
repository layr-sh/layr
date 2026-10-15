
# CustomDomain


## Properties

Name | Type
------------ | -------------
`createdAt` | Date
`domain` | string
`id` | any
`requiredDnsRecords` | [Array&lt;RequiredDNSRecord&gt;](RequiredDNSRecord.md)
`status` | string
`tlsAcmeEmail` | string
`tlsEnabled` | boolean
`tlsErrorMessage` | string
`tlsExpiresAt` | Date
`tlsIssuedAt` | Date
`tlsStatus` | string
`updatedAt` | Date

## Example

```typescript
import type { CustomDomain } from ''

// TODO: Update the object below with actual values
const example = {
  "createdAt": null,
  "domain": null,
  "id": null,
  "requiredDnsRecords": null,
  "status": null,
  "tlsAcmeEmail": null,
  "tlsEnabled": null,
  "tlsErrorMessage": null,
  "tlsExpiresAt": null,
  "tlsIssuedAt": null,
  "tlsStatus": null,
  "updatedAt": null,
} satisfies CustomDomain

console.log(example)

// Convert the instance to a JSON string
const exampleJSON: string = JSON.stringify(example)
console.log(exampleJSON)

// Parse the JSON string back to an object
const exampleParsed = JSON.parse(exampleJSON) as CustomDomain
console.log(exampleParsed)
```

[[Back to top]](#) [[Back to API list]](../README.md#api-endpoints) [[Back to Model list]](../README.md#models) [[Back to README]](../README.md)


