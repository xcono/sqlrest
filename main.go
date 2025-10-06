package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"sort"

	"github.com/urfave/cli/v2"
	"github.com/xcono/sqlrest/schema"
	"github.com/zeromicro/go-zero/core/conf"

	// mysql driver
	_ "github.com/go-sql-driver/mysql"
	"github.com/xcono/sqlrest/web"
)

var configFile = flag.String("f", "config.yaml", "the config file")

func main() {

	flag.Parse()

	var c schema.Config
	conf.MustLoad(*configFile, &c)

	app := &cli.App{
		Name:  "schema",
		Usage: "MySQL REST",
		Commands: []*cli.Command{
			{
				Name:  "start",
				Usage: "Start serving service",
				Args:  true,
				Action: func(cmd *cli.Context) error {

					web.StartServer(c) //blocking call
					return nil
				},
			},
			{
				Name:  "inspect",
				Usage: "Inspect service",
				Args:  true,
				Action: func(cmd *cli.Context) error {

					service := cmd.Args().Get(0)
					serviceConfig := c.Services[service]

					tablenames := make([]string, 0, len(serviceConfig.Schemas))
					for key, schema := range serviceConfig.Schemas {
						// by default: schema name == name name
						name := key
						// if altered table name, use it
						if schema.Table != "" {
							name = schema.Table
						}
						tablenames = append(tablenames, name)
					}

					// open db
					db, err := schema.OpenDB(serviceConfig.DSN)
					if err != nil {
						return err
					}
					defer db.Close()

					// get tables
					tables, err := schema.NewMySQL(db).Tables(tablenames...)
					if err != nil {
						return err
					}

					// pretty print tables as json
					jsonData, err := json.MarshalIndent(tables, "", "  ")
					if err != nil {
						return err
					}
					fmt.Println(string(jsonData))

					return nil
				},
			},
		},
	}

	sort.Sort(cli.FlagsByName(app.Flags))
	sort.Sort(cli.CommandsByName(app.Commands))

	if err := app.Run(os.Args); err != nil {
		panic(err)
	}
}
