# idmint

## Overview

`idmint` is a Go module for minting unique, time-sortable IDs in a distributed system without coordination between 
nodes. It is fully compatible with the [encoding/json](https://pkg.go.dev/encoding/json) package within the Go standard 
library, implementing the `json.Marshaler` and `json.Unmarshaler` interfaces for seamless integration into APIs.

## Usage

### Creating a minter

As per the Go proverb, the zero value of the `idmint.Minter` is useful, and you can simply create
an `idmint.Minter` as follows and begin using it:

```go
var minter idmint.Minter
```

However, if you have multiple nodes that will be minting IDs, you must create an `idmint.Minter` using the 
`idmint.NewMinter` function and pass a unique worker ID:

```go
var workerID string

// ...
// compute a unique worker id
// ...

minter, err := idmint.NewMinter(workerID)
if err != nil {
	// handle error
}
```

You can also configure the behaviour of an `idmint.Minter` by providing some `idmint.Configurer`'s to the
`idmint.NewMinter` function. For example:

```go
var workerID string

// ...
// compute a unique worker id
// ...

now := time.Now()
minter, err := idmint.NewMinter(
	workerID, 
	idmint.WithEpoch(now),
	
)
if err != nil {
	// handle error
}
```

### Minting an ID

Once you have created a `idmint.Minter`, you can call `idmint.Minter.Mint` to mint an ID. It requires you to specify 
what 'kind' of ID you would like to mint:

```go
var workerID string

// ...
// compute a unique worker id
// ...

minter, err := idmint.NewMinter(workerID)
if err != nil {
    // handle error
}

id, err := minter.Mint("user")
if err != nil {
    // handle error
}

fmt.Println(id) // e.g. "user_15099494400000"
```

> Specifying the kind of ID we are minting provides a human-readable way understand what kind of 'resource' an ID 
> is associated with.

## Documentation

Documentation for `idmint` can be found [here](https://pkg.go.dev/github.com/jordanhasgul/idmint).