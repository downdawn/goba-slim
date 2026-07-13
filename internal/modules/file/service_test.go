package file

import (
	"bytes"
	"context"
	"io"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

var (
	testOwner  = uuid.MustParse("0198a1d1-2c3b-7abc-8def-0123456789ab")
	testObject = uuid.MustParse("0198a1d1-2c3b-7abc-8def-1123456789ab")
)

func TestServiceUploadOpenAndDelete(t *testing.T) {
	store := newStartedStore(t)
	service := newTestService(t, store, false, 1024)
	png := append([]byte("\x89PNG\r\n\x1a\n"), bytes.Repeat([]byte{0}, 32)...)

	uploaded, err := service.Upload(t.Context(), testOwner, bytes.NewReader(png))
	require.NoError(t, err)
	require.Equal(t, ObjectKey{OwnerID: testOwner, ObjectID: testObject, Ext: "png"}, uploaded.Key)
	require.Equal(t, "image/png", uploaded.ContentType)
	require.Equal(t, int64(len(png)), uploaded.Size)
	require.Equal(t, "/files/"+uploaded.Key.String(), uploaded.URL())

	metadata, object, err := service.Open(t.Context(), uploaded.Key.String())
	require.NoError(t, err)
	t.Cleanup(func() { _ = object.Content.Close() })
	content, err := io.ReadAll(object.Content)
	require.NoError(t, err)
	require.Equal(t, png, content)
	require.Equal(t, "image/png", metadata.ContentType)

	require.ErrorIs(t, service.Delete(t.Context(), uuid.New(), false, uploaded.Key.String()), ErrForbidden)
	require.NoError(t, service.Delete(t.Context(), testOwner, false, uploaded.Key.String()))
	_, _, err = service.Open(t.Context(), uploaded.Key.String())
	require.ErrorIs(t, err, ErrNotFound)
}

func TestServiceSuperuserCanDeleteOtherUsersFile(t *testing.T) {
	store := newStartedStore(t)
	service := newTestService(t, store, false, 1024)
	png := append([]byte("\x89PNG\r\n\x1a\n"), bytes.Repeat([]byte{0}, 32)...)
	uploaded, err := service.Upload(t.Context(), testOwner, bytes.NewReader(png))
	require.NoError(t, err)

	require.NoError(t, service.Delete(t.Context(), uuid.New(), true, uploaded.Key.String()))
}

func TestServiceRejectsUnsupportedAndDisabledVideo(t *testing.T) {
	store := newStartedStore(t)
	service := newTestService(t, store, false, 1024)

	_, err := service.Upload(t.Context(), testOwner, bytes.NewReader([]byte("<svg xmlns='http://www.w3.org/2000/svg'/>")))
	require.ErrorIs(t, err, ErrTypeNotAllowed)

	mp4 := make([]byte, 32)
	copy(mp4[4:], []byte("ftypisom"))
	_, err = service.Upload(t.Context(), testOwner, bytes.NewReader(mp4))
	require.ErrorIs(t, err, ErrTypeNotAllowed)
}

func TestServiceRejectsEmptyFile(t *testing.T) {
	store := newStartedStore(t)
	service := newTestService(t, store, false, 1024)

	_, err := service.Upload(t.Context(), testOwner, bytes.NewReader(nil))
	require.ErrorIs(t, err, ErrInvalidFile)
}

func TestServiceRejectsOversizedFileAndCleansTemporaryObject(t *testing.T) {
	store := newStartedStore(t)
	service := newTestService(t, store, false, 512)
	png := append([]byte("\x89PNG\r\n\x1a\n"), bytes.Repeat([]byte{0}, 505)...)

	_, err := service.Upload(t.Context(), testOwner, bytes.NewReader(png))
	require.ErrorIs(t, err, ErrTooLarge)

	entries, readErr := store.root.ReadFile(testOwner.String() + "/." + testObject.String() + ".tmp")
	require.Error(t, readErr)
	require.Empty(t, entries)
	_, statErr := store.root.Stat(testOwner.String() + "/" + testObject.String() + ".png")
	require.Error(t, statErr)
}

func TestParseObjectKeyRejectsUnsafeValues(t *testing.T) {
	valid := testOwner.String() + "/" + testObject.String() + ".png"
	parsed, err := ParseObjectKey(valid)
	require.NoError(t, err)
	require.Equal(t, valid, parsed.String())

	for _, value := range []string{
		"", "../" + valid, testOwner.String() + "/../file.png",
		testOwner.String() + "\\" + testObject.String() + ".png",
		testOwner.String() + "/" + testObject.String() + ".svg",
		stringsToUpper(valid), testOwner.String() + "/" + testObject.String() + ".png/extra",
	} {
		_, err := ParseObjectKey(value)
		require.ErrorIs(t, err, ErrInvalidKey, value)
	}
}

func FuzzParseObjectKey(f *testing.F) {
	f.Add(testOwner.String() + "/" + testObject.String() + ".png")
	f.Add("../../etc/passwd")
	f.Add("C:\\Windows\\System32\\drivers\\etc\\hosts")
	f.Fuzz(func(t *testing.T, value string) {
		key, err := ParseObjectKey(value)
		if err != nil {
			return
		}
		require.Equal(t, value, key.String())
		require.NoError(t, key.Validate())
	})
}

func newStartedStore(t *testing.T) *LocalStore {
	t.Helper()
	store, err := NewLocalStore(t.TempDir())
	require.NoError(t, err)
	require.NoError(t, store.Start(t.Context()))
	t.Cleanup(func() { require.NoError(t, store.Stop(context.Background())) })
	return store
}

func newTestService(t *testing.T, store ObjectStore, video bool, imageMax int64) *Service {
	t.Helper()
	service, err := NewService(store, fixedID{value: testObject}, Limits{ImageMaxBytes: imageMax, VideoEnabled: video, VideoMaxBytes: 2048})
	require.NoError(t, err)
	return service
}

type fixedID struct {
	value uuid.UUID
	err   error
}

func (f fixedID) New() (uuid.UUID, error) { return f.value, f.err }

func stringsToUpper(value string) string {
	result := []byte(value)
	for index, char := range result {
		if char >= 'a' && char <= 'f' {
			result[index] = char - ('a' - 'A')
		}
	}
	return string(result)
}
