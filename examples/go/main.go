package main

import (
	"github.com/hctamu/pulumi-pve/sdk/go/pve/pool"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

func main() {
	pulumi.Run(func(ctx *pulumi.Context) error {
		_, err := pool.NewPool(ctx, "myPool", &pool.PoolArgs{
			Name:    pulumi.String("myPool"),
			Comment: pulumi.String("myPool").ToStringPtrOutput(),
		})
		if err != nil {
			return err
		}

		return nil
	})
}
