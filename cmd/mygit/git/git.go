package git

import (
	"bufio"
	"compress/zlib"
	"crypto/sha1"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"io/ioutil"
	"os"
	"path"
	"sort"
	"strings"
)

/*
	Unless Go adds the writers to the io/fs package, it's quite hard to use io/fs here...
*/

type Error string

func (e Error) Error() string {
	return string(e)
}

const (
	ErrRepositoryAlreadyInitialized = Error("repository already initialized")
	ErrInvalidHash                  = Error("invalid hash")
)

type Repository struct {
	root        string
	initialized bool
}

func NewRepository(root string) Repository {
	return Repository{root: root}
}

func (r *Repository) Init() (func() error, error) {
	cleanup := func() error {
		err := os.RemoveAll(path.Join(r.root, ".git"))
		if err != nil {
			return fmt.Errorf("error cleaning up: %w", err)
		}

		return nil
	}

	if r.initialized {
		return cleanup, ErrRepositoryAlreadyInitialized
	}

	dirs := []string{
		path.Join(r.root, ".git"),
		path.Join(r.root, ".git/objects"),
		path.Join(r.root, ".git/refs"),
	}
	for _, dir := range dirs {
		err := os.MkdirAll(dir, 0755)
		if err != nil {
			return cleanup, fmt.Errorf("error creating directory %s: %w", dir, err)
		}

	}

	filePath := path.Join(r.root, ".git/HEAD")
	headFileContents := []byte("ref: refs/heads/master\n")
	err := os.WriteFile(filePath, headFileContents, 0644)
	if err != nil {
		return cleanup, fmt.Errorf("error writing to file %s: %w", filePath, err)
	}

	r.initialized = true
	return cleanup, err
}

func (r *Repository) CatFile(hash string) (string, error) {
	isValid := len([]byte(hash)) == 40
	if !isValid {
		return "", fmt.Errorf("%w expected 40 characters, got: %d", ErrInvalidHash, len(hash))
	}

	blobPath := path.Join(r.root, ".git", "objects", hash[:2], hash[2:])
	blobFile, err := os.Open(blobPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return "", fmt.Errorf("object: %s does not exist", blobPath)
		}

		return "", fmt.Errorf("failed to open file: %w", err)
	}
	defer blobFile.Close()

	reader, err := zlib.NewReader(blobFile)
	if err != nil {
		return "", fmt.Errorf("failed to read the contents: %w", err)
	}
	defer reader.Close()

	blob, err := ioutil.ReadAll(reader)
	if err != nil {
		return "", fmt.Errorf("failed to read the contents: %w", err)
	}

	contents := string(blob)
	return strings.Split(contents, "\x00")[1], nil
}

func (r *Repository) WriteBlob(fs fs.FS, filename string) (string, error) {
	hash, blob, err := r.hashBlob(fs, filename)
	if err != nil {
		return "", fmt.Errorf("failed to hash the file: %w", err)
	}

	dirPath := path.Join(r.root, ".git/objects", hash[:2])
	err = os.MkdirAll(dirPath, 0755)
	if err != nil {
		return "", fmt.Errorf("failed to create the directory: %w", err)
	}

	blobFile, err := os.Create(path.Join(dirPath, hash[2:]))
	if err != nil {
		return "", fmt.Errorf("failed to create the file: %w", err)
	}

	w := zlib.NewWriter(blobFile)
	/*
		Remember to close BEFORE you read the contents of the file
	*/
	defer w.Close()

	_, err = w.Write(blob)
	if err != nil {
		return "", fmt.Errorf("failed to compress the contents: %w", err)
	}

	return hash, nil
}

func (r *Repository) ReadTree(hash string) (string, error) {
	isValid := len([]byte(hash)) == 40
	if !isValid {
		return "", fmt.Errorf("%w expected 40 characters, got: %d", ErrInvalidHash, len(hash))
	}

	blobPath := path.Join(r.root, ".git", "objects", hash[:2], hash[2:])
	blobFile, err := os.Open(blobPath)
	if err != nil {
		return "", fmt.Errorf("failed to open file: %w", err)
	}
	defer blobFile.Close()

	reader, err := zlib.NewReader(blobFile)
	if err != nil {
		if err != nil {
			return "", fmt.Errorf("failed to read the contents: %w", err)
		}
	}
	defer reader.Close()

	br := bufio.NewReader(reader)
	typ, err := br.ReadString(' ')
	if err != nil {
		return "", fmt.Errorf("error reading type: %w", err)
	}
	if typ != "tree " {
		return "", fmt.Errorf("expected type to be tree, got: %s", typ)
	}

	_, err = br.ReadString('\x00')
	if err != nil {
		return "", fmt.Errorf("error reading null byte: %w", err)
	}

	var names []string
	for {
		_, err = br.Peek(1)
		if err != nil {
			if err == io.EOF {
				break
			}

			return "", fmt.Errorf("error peeking: %w", err)
		}

		_, err = br.ReadString(' ')
		if err != nil {
			return "", fmt.Errorf("error reading mode: %w", err)
		}

		name, err := br.ReadString('\x00')
		if err != nil {
			return "", fmt.Errorf("error reading name: %w", err)
		}
		names = append(names, strings.Split(name, "\x00")[0])

		_, err = br.Read(make([]byte, 20))
		if err != nil {
			return "", fmt.Errorf("error reading hash: %w", err)
		}
	}

	contents := strings.Join(sort.StringSlice(names), "\n") + "\n"
	return contents, nil
}

func (r *Repository) WriteTree(dirname string) (string, error) {
	treeTable, err := r.treeTable(dirname)
	if err != nil {
		return "", fmt.Errorf("failed to hash the tree: %w", err)
	}

	output := fmt.Appendf([]byte{}, "tree %d\x00%s", len(treeTable), treeTable)
	if err != nil {
		return "", fmt.Errorf("failed to copy the contents: %w", err)
	}

	hash := fmt.Sprintf("%x", sha1.Sum(output))

	dirPath := path.Join(r.root, ".git/objects", hash[:2])
	err = os.MkdirAll(dirPath, 0755)
	if err != nil {
		return "", fmt.Errorf("failed to create the directory: %w", err)
	}

	treeFile, err := os.Create(path.Join(dirPath, hash[2:]))
	if err != nil {
		return "", fmt.Errorf("failed to create the file: %w", err)
	}

	w := zlib.NewWriter(treeFile)
	defer w.Close()

	_, err = w.Write(output)
	if err != nil {
		return "", fmt.Errorf("failed to compress the contents: %w", err)
	}

	return hash, nil
}

func (r *Repository) hashBlob(fsys fs.FS, filename string) (string, []byte, error) {
	file, err := fsys.Open(filename)
	if err != nil {
		return "", []byte{}, fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	info, err := file.Stat()
	if err != nil {
		return "", []byte{}, fmt.Errorf("failed to get file info: %w", err)
	}

	fBuf, err := ioutil.ReadAll(file)
	if err != nil {
		return "", []byte{}, fmt.Errorf("failed to copy the contents: %w", err)
	}

	header := append([]byte(fmt.Sprintf("blob %d", info.Size())), byte(0))
	blob := append(header, fBuf...)

	hash := fmt.Sprintf("%x", sha1.Sum(blob))
	return hash, blob, nil
}

func (r *Repository) treeTable(dirname string) (string, error) {
	dirEntries, err := os.ReadDir(dirname)
	if err != nil {
		return "", fmt.Errorf("failed to read the directory: %w", err)
	}

	var table []byte
	for _, dirEntry := range dirEntries {
		if dirEntry.IsDir() {
			if dirEntry.Name() == ".git" {
				continue
			}

			subHash, err := r.treeTable((path.Join(dirname, dirEntry.Name())))
			if err != nil {
				return "", fmt.Errorf("failed to write the tree: %w", err)
			}

			table = fmt.Appendf(table, "40000 %s\x00%x", dirEntry.Name(), sha1.Sum([]byte(subHash)))

		} else {
			hash, _, err := r.hashBlob(os.DirFS(dirname), dirEntry.Name())
			if err != nil {
				return "", fmt.Errorf("failed to hash the file: %w", err)
			}

			table = fmt.Appendf(table, "100644 %s\x00%s", dirEntry.Name(), hash)
		}
	}

	return string(table), nil
}
