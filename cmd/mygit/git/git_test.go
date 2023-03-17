package git_test

import (
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path"
	"testing"
	"testing/fstest"

	"github.com/codecrafters-io/git-starter-go/cmd/mygit/git"
)

func TestInitialize(t *testing.T) {
	t.Run("Initializes the repository with the right files", func(t *testing.T) {
		root := os.TempDir()

		repository := git.NewRepository(root)
		cleanup, err := repository.Init()
		defer cleanup()
		if err != nil {
			t.Fatalf("error initializing repository: %v", err)
		}

		fsys := os.DirFS(path.Join(root, ".git"))
		err = fstest.TestFS(fsys, "HEAD")
		if err != nil {
			t.Fatalf("error testing filesystem: %v", err)
		}

		f, err := fsys.Open("HEAD")
		if err != nil {
			t.Fatalf("error opening file: %v", err)
		}

		contents, err := io.ReadAll(f)
		if err != nil {
			t.Fatalf("error reading file: %v", err)
		}

		expected := []byte("ref: refs/heads/master\n")
		if string(contents) != string(expected) {
			t.Fatalf("expected %s, got %s", expected, contents)
		}
	})

	t.Run("Cannot initialize the repository twice", func(t *testing.T) {
		root := os.TempDir()
		repository := git.NewRepository(root)

		cleanup, err := repository.Init()
		defer cleanup()
		if err != nil {
			t.Fatalf("error initializing repository: %v", err)
		}

		_, err = repository.Init()
		if err == nil {
			t.Fatalf("expected error when initializing the repository the second time, got nil")
		}

		if !errors.Is(err, git.ErrRepositoryAlreadyInitialized) {
			t.Fatalf("expected error %v, got %v", git.ErrRepositoryAlreadyInitialized, err)
		}
	})
}

func TestCatFile(t *testing.T) {
	t.Run("Fails to read the blob if the SHA is invalid", func(t *testing.T) {
		root := os.TempDir()
		repository := git.NewRepository(root)

		_, err := repository.CatFile("123")
		if err == nil {
			t.Fatalf("expected error when reading sha, got nil")
		}

		if !errors.Is(err, git.ErrInvalidHash) {
			t.Fatalf("expected error %v, got %v", git.ErrInvalidHash, err)
		}
	})

	t.Run("Reads the blob", func(t *testing.T) {
		root := os.TempDir()

		repository := git.NewRepository(root)
		cleanup, err := repository.Init()
		defer cleanup()
		if err != nil {
			t.Fatalf("error initializing repository: %v", err)
		}

		wd, err := os.Getwd()
		if err != nil {
			t.Fatalf("error getting working directory: %v", err)
		}

		const blobSha = "d670460b4b4aece5915caf5c68d12f560a9fe3e4"
		cmd := exec.Command("cp", "-r", path.Join(wd, "./fixtures", blobSha[:2]), path.Join(root, ".git/objects"))
		err = cmd.Run()
		if err != nil {
			t.Fatalf("error copying testdata: %v", err)
		}

		contents, err := repository.CatFile(blobSha)
		if err != nil {
			t.Fatalf("error reading blob: %v", err)
		}

		if contents != "test content\n" {
			t.Fatalf("expected test content, got %s", contents)
		}
	})
}

func TestHashFile(t *testing.T) {
	t.Run("succeeds", func(t *testing.T) {
		root := os.TempDir()
		repository := git.NewRepository(root)

		fileContents := "test content"

		fsys := os.DirFS(path.Join(root))
		filename := "test.txt"
		err := os.WriteFile(path.Join(root, filename), []byte(fileContents), 0644)
		if err != nil {
			t.Fatalf("error writing file: %v", err)
		}
		defer os.RemoveAll(path.Join(root, filename))
		defer cleanup(t, root)

		hash, err := repository.WriteBlob(fsys, filename)
		if err != nil {
			t.Fatalf("error hashing file: %v", err)
		}

		dirPath := path.Join(root, ".git", "objects", hash[:2])
		fi, err := os.Stat(dirPath)
		if err != nil {
			t.Fatalf("error stating directory: %v", err)
		}

		if !fi.IsDir() {
			t.Fatalf("expected %s to be a directory", dirPath)
		}

		blobPath := path.Join(dirPath, hash[2:])
		fi, err = os.Stat(blobPath)
		if err != nil {
			t.Fatalf("error stating file: %v", err)
		}

		if fi.IsDir() {
			t.Fatalf("expected %s to be a file", blobPath)
		}

		contents, err := repository.CatFile(hash)
		if err != nil {
			t.Fatalf("error reading blob: %v", err)
		}

		if contents != fileContents {
			t.Fatalf("expected %s, got %s", fileContents, contents)
		}
	})
}

func TestReadTree(t *testing.T) {
	t.Run("succeeds", func(t *testing.T) {
		root := os.TempDir()

		repository := git.NewRepository(root)
		cleanup, err := repository.Init()
		defer cleanup()
		if err != nil {
			t.Fatalf("error initializing repository: %v", err)
		}

		wd, err := os.Getwd()
		if err != nil {
			t.Fatalf("error getting working directory: %v", err)
		}

		const blobSha = "03036dc311ab67d9ea0297eb2bfec564fdeb322f"
		cmd := exec.Command("cp", "-r", path.Join(wd, "./fixtures", blobSha[:2]), path.Join(root, ".git/objects"))
		err = cmd.Run()
		if err != nil {
			t.Fatalf("error copying testdata: %v", err)
		}

		output, err := repository.ReadTree(blobSha)
		if err != nil {
			t.Fatalf("error reading tree: %v", err)
		}

		fmt.Println(output)
	})
}

func cleanup(t *testing.T, p string) {
	t.Helper()
	err := os.RemoveAll(path.Join(p, ".git"))
	if err != nil {
		t.Fatalf("error cleaning up: %v", err)
	}
}
