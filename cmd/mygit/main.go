package main

import (
	"flag"
	"fmt"
	"os"
	"path"

	"github.com/codecrafters-io/git-starter-go/cmd/mygit/git"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintf(os.Stderr, "Missing commands")
		os.Exit(1)
	}

	root := flag.String("root", ".", "path to git repo")
	flag.Parse()

	err := run(*root, Command(flag.Arg(0)))
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %s", err)
		os.Exit(1)
	}
}

type Command string

const (
	Init       Command = "init"
	CatFile    Command = "cat-file"
	HashObject Command = "hash-object"
	LsTree     Command = "ls-tree"
	WriteTree  Command = "write-tree"
)

func run(root string, command Command) error {
	repository := git.NewRepository(root)
	if command == Init {
		_, err := repository.Init()
		return err
	}

	if command == CatFile {
		fs := flag.NewFlagSet("cat-file", flag.ContinueOnError)
		fsPrettyPrint := fs.String("p", "", "pretty print")
		err := fs.Parse(flag.Args()[1:])
		if err != nil {
			return err
		}

		if *fsPrettyPrint == "" {
			return fmt.Errorf("missing argument -p")
		}

		out, err := repository.CatFile(*fsPrettyPrint)
		if err != nil {
			return err
		}

		fmt.Print(out)
		return nil
	}

	if command == HashObject {
		fs := flag.NewFlagSet("hash-file", flag.ContinueOnError)
		fsWrite := fs.String("w", "", "write")
		err := fs.Parse(flag.Args()[1:])
		if err != nil {
			return err
		}

		if *fsWrite == "" {
			return fmt.Errorf("missing argument -w")
		}

		dir := path.Dir(*fsWrite)
		filename := path.Base(*fsWrite)
		fsys := os.DirFS(path.Dir(dir))

		hash, err := repository.WriteBlob(fsys, filename)
		if err != nil {
			return err
		}

		fmt.Println(hash)
		return nil
	}

	if command == LsTree {
		fs := flag.NewFlagSet("ls-tree", flag.ContinueOnError)
		fsNameOnly := fs.String("name-only", "", "name only")
		err := fs.Parse(flag.Args()[1:])
		if err != nil {
			return err
		}

		if *fsNameOnly == "" {
			return fmt.Errorf("missing argument --name-only")
		}

		out, err := repository.ReadTree(*fsNameOnly)
		if err != nil {
			return err
		}

		fmt.Print(out)
		return nil
	}

	if command == WriteTree {
		out, err := repository.WriteTree(".")
		if err != nil {
			return err
		}

		fmt.Print(out)
		return nil
	}

	return fmt.Errorf("not implemented %s", command)
}
