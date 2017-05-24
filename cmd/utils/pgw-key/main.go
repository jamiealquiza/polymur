package main

import (
	"crypto/sha256"
	"fmt"
	"math/rand"
	"os"
	"time"

	"github.com/jamiealquiza/consul/api"
	"github.com/jamiealquiza/polymur/keysync"
)

// Commands available.
var commands = map[string]func(*api.KV, *keysync.APIKeys, []string){
	"list":   list,
	"create": create,
	"regen":  regen,
	"delete": kdelete,
}

// Subcommand names and descriptions.
var subCommands = map[string]map[string]string{
	"list": map[string]string{
		"names": "\tShow API key names",
		"all":   "\tShow API key names and key",
	},
	"create": map[string]string{
		"<argument>": "\tCreate a new API user & key where <argument> becomes the user name",
	},
	"regen": map[string]string{
		"<argument>": "\tRegenerate a key for the existing user specified",
	},
	"delete": map[string]string{
		"<argument>": "\tDelete unregisters a key for the specified user",
	},
}

func main() {
	// Handle input.
	if len(os.Args) < 2 {
		showCommands()
		os.Exit(1)
	}

	cmd, valid := commands[os.Args[1]]
	if !valid {
		fmt.Printf("Not a valid command: %s\n\n", os.Args[1])
		showCommands()
		os.Exit(1)
	}

	// No arguments following command,
	// just dump help info.
	if len(os.Args) < 3 {
		cmd(nil, nil, nil)
		return
	}

	// Otherwise hit Consul and
	// run the command.
	keys := keysync.NewAPIKeys()

	client, err := api.NewClient(api.DefaultConfig())
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	kv := client.KV()

	registeredKeys, _, err := kv.List("/polymur/gateway/keys", nil)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	_, _ = keysync.Sync(keys, registeredKeys)

	// Pass the command only arguments.
	cmd(kv, keys, os.Args[2:])
}

// list returns information about registered
// API key names and keys.
func list(kv *api.KV, k *keysync.APIKeys, args []string) {
	if len(args) == 0 {
		printHelp("list")
	}

	if _, exists := subCommands["list"][args[0]]; !exists {
		fmt.Printf("pgw-key list %s is invalid\n", args[0])
		printHelp("list")
	}

	switch args[0] {
	case "names":
		for _, v := range k.Keys {
			fmt.Printf("%s\n", v)
		}
	case "all":
		for k, v := range k.Keys {
			fmt.Printf("%s: %s\n", v, k)
		}
	}
}

// create takes a key name, generates a key and
// registers it in Consul.
func create(kv *api.KV, k *keysync.APIKeys, args []string) {
	if len(args) == 0 {
		printHelp("create")
	}

	keyName := args[0]

	// Check if the key name is already used.
	if k.KeyNameExists(keyName) {
		fmt.Printf("Key %s already exists\n", keyName)
		os.Exit(0)
	}

	key := genKey(keyName)

	// Register the key in Consul.
	p := &api.KVPair{Key: "polymur/gateway/keys/" + keyName, Value: []byte(key)}
	_, err := kv.Put(p, nil)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	fmt.Printf("Successfully registered %s with key %s\n", keyName, key)
}

// regen takes a key name and creates a new key
// and updates the entry in Consul.
func regen(kv *api.KV, k *keysync.APIKeys, args []string) {
	if len(args) == 0 {
		printHelp("regen")
	}

	keyName := args[0]

	// Check if the key exists.
	if !k.KeyNameExists(keyName) {
		fmt.Printf("Key %s doesn't exist\n", keyName)
		os.Exit(0)
	}

	key := genKey(keyName)

	// Updated the key in Consul.
	p := &api.KVPair{Key: "polymur/gateway/keys/" + keyName, Value: []byte(key)}
	_, err := kv.Put(p, nil)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	fmt.Printf("Successfully regenerated %s with key %s\n", keyName, key)
}

// kdelete removes a registered key in Consul.
func kdelete(kv *api.KV, k *keysync.APIKeys, args []string) {
	if len(args) == 0 {
		printHelp("delete")
	}

	keyName := args[0]

	// Check if the key name exists.
	if !k.KeyNameExists(keyName) {
		fmt.Printf("Key %s doesn't exist\n", keyName)
		os.Exit(0)
	}

	_, err := kv.Delete("polymur/gateway/keys/"+keyName, nil)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	fmt.Printf("Successfully deleted %s\n", keyName)
}

// genKey takes a keyname and generates
// a random key string.
func genKey(keyName string) string {
	// Don't even ask.
	gen := rand.New(rand.NewSource(int64(time.Now().Unix()) / int64(os.Getpid())))
	nameHash := fmt.Sprintf("%x", sha256.Sum256([]byte(keyName)))
	hashString := fmt.Sprintf("%d%s", gen.Int63(), nameHash)

	keyArray := [24]byte{}
	for i := 0; i < 24; i++ {
		keyArray[i] = hashString[gen.Intn(len(hashString))]
	}

	return fmt.Sprintf("%s", keyArray)
}

// showCommands shows all of the available
// pgw-key commands.
func showCommands() {
	fmt.Println("Commands ('pgw-key <command>' for help):")
	for k, _ := range commands {
		fmt.Printf("\t%s\n", k)
	}

}

// printHelp shows subcommands and descriptions
// available for each pgw-key command.
func printHelp(s string) {
	fmt.Printf("pgw-key %s <argument>:\n", s)
	for k, v := range subCommands[s] {
		fmt.Printf("\t%s:%s\n", k, v)
	}

	os.Exit(0)
}
