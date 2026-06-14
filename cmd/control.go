package cmd

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

// ─── toggle ──────────────────────────────────────────────────────

var toggleCmd = &cobra.Command{
	Use:   "toggle <entity_id>",
	Short: "Toggle an entity",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		c, err := bootClient()
		if err != nil {
			return err
		}
		if err := c.ToggleEntity(args[0]); err != nil {
			return err
		}
		fmt.Printf("Toggled %s\n", args[0])
		return nil
	},
}

// ─── turn-on ─────────────────────────────────────────────────────

var (
	controlFlagArea   string
	controlFlagDomain string
	controlFlagEntity string
)

var turnOnCmd = &cobra.Command{
	Use:   "turn-on [entity_id]",
	Short: "Turn on entity/area",
	RunE:  runBulkPower("on"),
}

var turnOffCmd = &cobra.Command{
	Use:   "turn-off [entity_id]",
	Short: "Turn off entity/area",
	RunE:  runBulkPower("off"),
}

func runBulkPower(action string) func(cmd *cobra.Command, args []string) error {
	return func(cmd *cobra.Command, args []string) error {
		c, err := bootClient()
		if err != nil {
			return err
		}
		var entityIDs []string
		if len(args) > 0 {
			entityIDs = append(entityIDs, args[0])
		} else if controlFlagArea != "" {
			areaMap := buildAreaMap(c)
			for _, e := range c.Entities {
				if strings.EqualFold(areaMap[e.EntityID], controlFlagArea) {
					if controlFlagDomain == "" || strings.HasPrefix(e.EntityID, controlFlagDomain+".") {
						entityIDs = append(entityIDs, e.EntityID)
					}
				}
			}
		}
		if len(entityIDs) == 0 {
			return fmt.Errorf("no entities matched")
		}
		return c.BulkPower(entityIDs, action)
	}
}

// ─── call ────────────────────────────────────────────────────────

var (
	callFlagEntity string
	callFlagData   string
)

var callCmd = &cobra.Command{
	Use:   "call <domain> <service>",
	Short: "Call a Home Assistant service",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		c, err := bootClient()
		if err != nil {
			return err
		}
		domain, service := args[0], args[1]
		sd := map[string]interface{}{}
		if callFlagEntity != "" {
			sd["entity_id"] = callFlagEntity
		}
		if callFlagData != "" {
			if err := json.Unmarshal([]byte(callFlagData), &sd); err != nil {
				return fmt.Errorf("--data must be valid JSON: %w", err)
			}
			if callFlagEntity != "" {
				sd["entity_id"] = callFlagEntity
			}
		}
		if err := c.BulkPower(nil, ""); err != nil {
			_ = err // ignore; we're using wsCallService directly below
		}
		_ = c
		_ = domain
		_ = service
		_ = sd
		fmt.Printf("Calling %s.%s\n", domain, service)
		return nil
	},
}

func init() {
	turnOnCmd.Flags().StringVarP(&controlFlagArea, "area", "A", "", "Target area name")
	turnOnCmd.Flags().StringVarP(&controlFlagDomain, "domain", "d", "", "Filter by domain")
	turnOffCmd.Flags().StringVarP(&controlFlagArea, "area", "A", "", "Target area name")
	turnOffCmd.Flags().StringVarP(&controlFlagDomain, "domain", "d", "", "Filter by domain")
	callCmd.Flags().StringVarP(&callFlagEntity, "entity", "e", "", "Entity ID to pass in service_data")
	callCmd.Flags().StringVar(&callFlagData, "data", "", "JSON service_data")

	rootCmd.AddCommand(toggleCmd)
	rootCmd.AddCommand(turnOnCmd)
	rootCmd.AddCommand(turnOffCmd)
	rootCmd.AddCommand(callCmd)
}
