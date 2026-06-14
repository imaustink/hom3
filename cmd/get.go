package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"text/tabwriter"

	"github.com/imaustink/hom3/internal/client"
	"github.com/imaustink/hom3/internal/color"
	"github.com/imaustink/hom3/internal/config"
	"github.com/imaustink/hom3/internal/model"
	"github.com/spf13/cobra"
)

var getCmd = &cobra.Command{
	Use:   "get <resource>",
	Short: "Retrieve Home Assistant resources",
	Long: `Retrieve entities, areas, or devices from Home Assistant.

Resources:
  entities    All (or filtered) entity states
  entity      Single entity by entity_id
  areas       Area registry
  devices     Device registry`,
}

// ─── get entities ────────────────────────────────────────────────

var (
	getFlagDomain string
	getFlagSearch string
	getFlagArea   string
	getFlagOutput string
)

var getEntitiesCmd = &cobra.Command{
	Use:   "entities",
	Short: "List entity states",
	RunE: func(cmd *cobra.Command, args []string) error {
		c, err := bootClient()
		if err != nil {
			return err
		}

		entities := clientEntities(c)

		// Apply filters
		areaMap := buildAreaMap(c)
		if getFlagDomain != "" || getFlagSearch != "" || getFlagArea != "" {
			view := model.DeviceTypeAll
			if getFlagDomain != "" {
				if dt, ok := model.DeviceTypeShortcuts[getFlagDomain]; ok {
					view = dt
				}
			}
			entities = filterEntitiesCLI(entities, view, getFlagSearch, areaMap, getFlagArea)
		}

		return printEntities(entities, areaMap, getFlagOutput)
	},
}

var getEntityCmd = &cobra.Command{
	Use:   "entity <entity_id>",
	Short: "Get a single entity",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		c, err := bootClient()
		if err != nil {
			return err
		}
		entityID := args[0]
		e, ok := c.Entities[entityID]
		if !ok {
			return fmt.Errorf("entity %q not found", entityID)
		}
		return printEntities([]model.HassEntity{*e}, nil, getFlagOutput)
	},
}

var getAreasCmd = &cobra.Command{
	Use:   "areas",
	Short: "List area registry",
	RunE: func(cmd *cobra.Command, args []string) error {
		c, err := bootClient()
		if err != nil {
			return err
		}
		if getFlagOutput == "json" {
			return printJSON(c.Areas)
		}
		w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		fmt.Fprintln(w, "AREA_ID\tNAME")
		for _, a := range c.Areas {
			fmt.Fprintf(w, "%s\t%s\n", a.AreaID, a.Name)
		}
		return w.Flush()
	},
}

var getDevicesCmd = &cobra.Command{
	Use:   "devices",
	Short: "List device registry",
	RunE: func(cmd *cobra.Command, args []string) error {
		c, err := bootClient()
		if err != nil {
			return err
		}
		if getFlagOutput == "json" {
			return printJSON(c.Devices)
		}
		w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		fmt.Fprintln(w, "DEVICE_ID\tNAME\tAREA_ID")
		for _, d := range c.Devices {
			areaID := ""
			if d.AreaID != nil {
				areaID = *d.AreaID
			}
			fmt.Fprintf(w, "%s\t%s\t%s\n", d.ID, d.Name, areaID)
		}
		return w.Flush()
	},
}

func init() {
	// entity filtering flags
	getEntitiesCmd.Flags().StringVarP(&getFlagDomain, "domain", "d", "", "Filter by device type / domain")
	getEntitiesCmd.Flags().StringVarP(&getFlagSearch, "search", "s", "", "Search substring")
	getEntitiesCmd.Flags().StringVarP(&getFlagArea, "area", "A", "", "Filter by area name")
	getEntitiesCmd.Flags().StringVarP(&getFlagOutput, "output", "o", "table", "Output format: table|wide|json")
	getEntityCmd.Flags().StringVarP(&getFlagOutput, "output", "o", "table", "Output format: table|wide|json")
	getAreasCmd.Flags().StringVarP(&getFlagOutput, "output", "o", "table", "Output format: table|wide|json")
	getDevicesCmd.Flags().StringVarP(&getFlagOutput, "output", "o", "table", "Output format: table|wide|json")

	getCmd.AddCommand(getEntitiesCmd)
	getCmd.AddCommand(getEntityCmd)
	getCmd.AddCommand(getAreasCmd)
	getCmd.AddCommand(getDevicesCmd)
	rootCmd.AddCommand(getCmd)
}

// ─────────────────────────────────────────────── Shared helpers ──

// bootClient creates a connected HassClient using the global flags / config.
func bootClient() (*client.HassClient, error) {
	homes, _, err := config.Load(flagURL, flagToken, flagName)
	if err != nil || len(homes) == 0 {
		return nil, fmt.Errorf("no configuration found")
	}
	c := client.New()
	if err := c.Connect(homes[0]); err != nil {
		return nil, fmt.Errorf("connect: %w", err)
	}
	return c, nil
}

func clientEntities(c *client.HassClient) []model.HassEntity {
	es := make([]model.HassEntity, 0, len(c.Entities))
	for _, e := range c.Entities {
		es = append(es, *e)
	}
	return es
}

func buildAreaMap(c *client.HassClient) map[string]string {
	// device → area name
	deviceArea := make(map[string]string)
	areaName := make(map[string]string)
	for _, a := range c.Areas {
		areaName[a.AreaID] = a.Name
	}
	for _, d := range c.Devices {
		if d.AreaID != nil {
			deviceArea[d.ID] = areaName[*d.AreaID]
		}
	}
	m := make(map[string]string)
	for _, reg := range c.EntityRegistry {
		if reg.AreaID != nil {
			m[reg.EntityID] = areaName[*reg.AreaID]
		} else if reg.DeviceID != nil {
			m[reg.EntityID] = deviceArea[*reg.DeviceID]
		}
	}
	return m
}

func filterEntitiesCLI(entities []model.HassEntity, view model.DeviceType, filter string, areaMap map[string]string, areaFilter string) []model.HassEntity {
	domains := model.DeviceTypeDomains[view]
	domainSet := make(map[string]bool)
	for _, d := range domains {
		domainSet[d] = true
	}
	filterLow := strings.ToLower(filter)
	areaLow := strings.ToLower(areaFilter)
	var result []model.HassEntity
	for _, e := range entities {
		if len(domains) > 0 {
			dot := strings.IndexByte(e.EntityID, '.')
			domain := e.EntityID
			if dot >= 0 {
				domain = e.EntityID[:dot]
			}
			if !domainSet[domain] {
				continue
			}
		}
		if areaLow != "" && !strings.Contains(strings.ToLower(areaMap[e.EntityID]), areaLow) {
			continue
		}
		if filterLow != "" {
			name := strings.ToLower(color.FriendlyName(e))
			if !strings.Contains(strings.ToLower(e.EntityID), filterLow) &&
				!strings.Contains(name, filterLow) &&
				!strings.Contains(strings.ToLower(e.State), filterLow) {
				continue
			}
		}
		result = append(result, e)
	}
	return result
}

func printEntities(entities []model.HassEntity, areaMap map[string]string, output string) error {
	if output == "json" {
		return printJSON(entities)
	}
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	if output == "wide" {
		fmt.Fprintln(w, "ENTITY_ID\tSTATE\tFRIENDLY_NAME\tAREA\tLAST_CHANGED")
		for _, e := range entities {
			area := ""
			if areaMap != nil {
				area = areaMap[e.EntityID]
			}
			fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n",
				e.EntityID, color.FormatState(e), color.FriendlyName(e), area, e.LastChanged)
		}
	} else {
		fmt.Fprintln(w, "ENTITY_ID\tSTATE\tFRIENDLY_NAME")
		for _, e := range entities {
			fmt.Fprintf(w, "%s\t%s\t%s\n",
				e.EntityID, color.FormatState(e), color.FriendlyName(e))
		}
	}
	return w.Flush()
}

func printJSON(v interface{}) error {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}
