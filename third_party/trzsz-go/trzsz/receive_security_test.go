package trzsz

import (
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func receivingTransfer(t *testing.T) *trzszTransfer {
	t.Helper()
	transfer := newTransfer(nil, nil)
	t.Cleanup(transfer.cleanup)
	return transfer
}

func TestReceiveRejectsInvalidPathComponents(t *testing.T) {
	for _, overwrite := range []bool{false, true} {
		for _, parts := range [][]string{
			nil, {""}, {"."}, {".."}, {"../victim"}, {"/victim"}, {`..\victim`},
			{"bundle", "..", "..", "victim"}, {"bundle", "/victim"}, {"bundle", "sub/victim"},
		} {
			dir := t.TempDir()
			transfer := receivingTransfer(t)
			transfer.transferConfig.Overwrite = overwrite
			writer, _, err := transfer.createDirOrFile(dir, &sourceFile{RelPath: parts}, true)
			if writer != nil {
				_ = writer.Close()
			}
			require.Error(t, err, "overwrite=%v path=%q", overwrite, parts)
		}
	}
	for _, name := range []string{"../victim", `..\victim`, "/victim", "."} {
		transfer := receivingTransfer(t)
		_, _, err := transfer.createFile(t.TempDir(), name, true, nil)
		require.Error(t, err, "legacy file name %q", name)
	}
}

func TestReceiveCannotOverwriteOutsideDirectory(t *testing.T) {
	base := t.TempDir()
	dest := filepath.Join(base, "downloads")
	require.NoError(t, os.Mkdir(dest, 0700))
	victim := filepath.Join(base, "victim")
	require.NoError(t, os.WriteFile(victim, []byte("original"), 0600))
	transfer := receivingTransfer(t)
	_, _, err := transfer.createDirOrFile(dest, &sourceFile{RelPath: []string{"bundle", "..", "..", "victim"}}, true)
	require.Error(t, err)
	got, err := os.ReadFile(victim)
	require.NoError(t, err)
	assert.Equal(t, "original", string(got))
}

func TestReceiveRejectsSymlinkEscapes(t *testing.T) {
	for _, overwrite := range []bool{false, true} {
		for _, linkParent := range []bool{false, true} {
			base := t.TempDir()
			dest := filepath.Join(base, "downloads")
			outside := filepath.Join(base, "outside")
			require.NoError(t, os.MkdirAll(filepath.Join(dest, "bundle"), 0700))
			require.NoError(t, os.Mkdir(outside, 0700))
			victim := filepath.Join(outside, "victim")
			require.NoError(t, os.WriteFile(victim, []byte("original"), 0600))
			link, target := filepath.Join(dest, "bundle", "link"), outside
			parts := []string{"bundle", "link", "victim"}
			if !linkParent {
				link, target = filepath.Join(dest, "bundle", "victim"), victim
				parts = []string{"bundle", "victim"}
			}
			if err := os.Symlink(target, link); err != nil {
				t.Skipf("symlinks unavailable: %v", err)
			}
			transfer := receivingTransfer(t)
			transfer.transferConfig.Overwrite = overwrite
			transfer.fileNameMap[0] = "bundle"
			_, _, err := transfer.createDirOrFile(dest, &sourceFile{RelPath: parts}, true)
			require.Error(t, err, "overwrite=%v parent link=%v", overwrite, linkParent)
			got, err := os.ReadFile(victim)
			require.NoError(t, err)
			assert.Equal(t, "original", string(got))
		}
	}
}

func TestReceiveAllowsInternalSymlinksAndUnicode(t *testing.T) {
	dest := t.TempDir()
	require.NoError(t, os.Mkdir(filepath.Join(dest, "real"), 0700))
	if err := os.Symlink("real", filepath.Join(dest, "link")); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	transfer := receivingTransfer(t)
	transfer.transferConfig.Overwrite = true
	writer, _, err := transfer.createDirOrFile(dest, &sourceFile{RelPath: []string{"link", "文件"}}, true)
	require.NoError(t, err)
	_, err = io.WriteString(writer, "content")
	require.NoError(t, err)
	require.NoError(t, writer.Close())
	got, err := os.ReadFile(filepath.Join(dest, "real", "文件"))
	require.NoError(t, err)
	assert.Equal(t, "content", string(got))
}

func TestReceiveCleanupCannotFollowReplacedDirectory(t *testing.T) {
	dest, outside := t.TempDir(), t.TempDir()
	transfer := receivingTransfer(t)
	transfer.transferConfig.Overwrite = true
	writer, _, err := transfer.createDirOrFile(dest, &sourceFile{RelPath: []string{"bundle", "victim"}}, true)
	require.NoError(t, err)
	require.NoError(t, writer.Close())
	require.NoError(t, os.WriteFile(filepath.Join(outside, "victim"), []byte("original"), 0600))
	require.NoError(t, os.Rename(filepath.Join(dest, "bundle"), filepath.Join(dest, "moved")))
	if err := os.Symlink(outside, filepath.Join(dest, "bundle")); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	transfer.deleteCreatedFiles()
	got, err := os.ReadFile(filepath.Join(outside, "victim"))
	require.NoError(t, err)
	assert.Equal(t, "original", string(got))
}

func TestArchiveClosesEveryMember(t *testing.T) {
	dest := t.TempDir()
	transfer := receivingTransfer(t)
	w, _, err := transfer.createDirOrFile(dest, &sourceFile{RelPath: []string{"bundle"}, IsDir: true, Archive: true}, true)
	require.NoError(t, err)
	writer := w.(*archiveFileWriter)
	t.Cleanup(func() { _ = writer.Close() })
	header := func(name string, size int64) string {
		src := &sourceFile{RelPath: []string{"bundle", name}, Size: size}
		json, err := src.marshalSourceFile()
		require.NoError(t, err)
		return encodeString(json) + "\n"
	}
	_, err = io.WriteString(writer, header("first", 1))
	require.NoError(t, err)
	first := writer.file.getFile()
	_, err = io.WriteString(writer, "x")
	require.NoError(t, err)
	// Start the next member with a fragmented header.
	next := header("empty", 0)
	_, err = io.WriteString(writer, next[:1])
	require.NoError(t, err)
	// Read reports os.ErrClosed on every platform; Stat may return a Windows handle error.
	_, err = first.Read(make([]byte, 1))
	assert.ErrorIs(t, err, os.ErrClosed)
	_, err = io.WriteString(writer, next[1:])
	require.NoError(t, err)
	empty := writer.file.getFile()
	_, err = io.WriteString(writer, header("last", 0))
	require.NoError(t, err)
	_, err = empty.Read(make([]byte, 1))
	assert.ErrorIs(t, err, os.ErrClosed)
	last := writer.file.getFile()
	require.NoError(t, writer.Close())
	require.NoError(t, writer.Close())
	_, err = last.Read(make([]byte, 1))
	assert.ErrorIs(t, err, os.ErrClosed)
}
