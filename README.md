# idmint

## Overview

`idmint` is a Go module for minting unique, time-sortable, IDs in a distributed system without coordination between 
nodes.

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
var workerID uint64

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
var workerID uint64

// ...
// compute a unique worker id
// ...

now := time.Now()
minter, err := idmint.NewMinter(
	workerID, 
	idmint.WithStartOfMintingTime(now),
)
if err != nil {
	// handle error
}
```

### Minting an ID

Once you have created an `idmint.Minter`, you can call `idmint.Minter.Mint` to mint an ID:

```go
var workerID uint64

// ...
// compute a unique worker id
// ...

minter, err := idmint.NewMinter(workerID)
if err != nil {
    // handle error
}

id, err := minter.Mint()
if err != nil {
    // handle error
}

fmt.Println(id) // e.g. 61898956800000
```

## Documentation

Documentation for `idmint` can be found [here](https://pkg.go.dev/github.com/jordanhasgul/idmint).
