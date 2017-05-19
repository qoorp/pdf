package add_obj

import (
	//	"fmt"
	"bytes"
	"io/ioutil"
	"testing"

	rscpdf "github.com/qoorp/pdf"
)

func Test_startxref(t *testing.T) {
	content, err := ioutil.ReadFile("Sample.pdf")
	if err != nil {
		t.Error("ReadFile", err)
	}
	pdf, err := NewPDF(content)
	if err != nil {
		t.Error("pdf", err)
	}
	if pdf.rscreader.Startxref() != 502 {
		t.Error("startxref expected 502 got", pdf.rscreader.Startxref())
	}
}

func Test_trailer(t *testing.T) {
	content, err := ioutil.ReadFile("Sample.pdf")
	if err != nil {
		t.Error("ReadFile", err)
	}
	pdf, err := NewPDF(content)
	if err != nil {
		t.Error("pdf", err)
	}
	tr := pdf.rscreader.Trailer()
	if tr.Key("Size").Int64() != 6 {
		t.Error("trailer expected 6 got", tr.Key("Size").Int64())
	}
}

func Test_stream(t *testing.T) {
	obj := 12
	c := "acontent\n"
	compressedlength := 21
	dict := rscpdf.ValueDict()
	dict.SetMapIndex("kalle", rscpdf.ValueString("gustav"))
	s, err := streamObj(obj, 0, []byte(c), dict)
	if err != nil {
		t.Error("stream got", err)
	}
	if s.obj != obj {
		t.Error("stream expected", obj, "got", s.obj)
	}
	v := s.dict
	l := v.Key("Length")
	if int(l.Int64()) != compressedlength {
		t.Error("stream expected", compressedlength, "got", l)
	}
	g := v.Key("kalle")
	if g.RawString() != "gustav" {
		t.Error("stream expected", "gustav", "got", g.RawString())
	}
}

func Test_pdf(t *testing.T) {
	content, err := ioutil.ReadFile("Sample.pdf")
	if err != nil {
		t.Error("ReadFile", err)
	}
	pdf, err := NewPDF(content)
	if err != nil {
		t.Error("pdf", err)
	}
	b := bytes.Buffer{}
	_, err = pdf.Write(&b)
	if err != nil {
		t.Error("Write", err)
	}
	us := b.String()
	expected := string(content)
	if us != expected {
		t.Error("stream expected", expected, "got", us)
	}
}

func Test_obj(t *testing.T) {
	p := pdfFromSample(t)
	var found []rscpdf.Value
	found = p.rscreader.Obj(4)
	if len(found) != 1 {
		t.Error("Obj expected 1 got", len(found))
	}
	v := found[0]
	font := v.Key("Type")
	if font.Name() != "Font" {
		t.Error("Obj expected Font got", font.Name())
	}
}

func Test_pdfaddstream(t *testing.T) {
	pdf1 := pdfFromSample(t)
	addedc := "some content"
	pdf1.AddStream([]byte(addedc), rscpdf.ValueDict())
	pdf2 := checkModification(t, pdf1, 788, 7)
	tr := pdf2.rscreader.Trailer()
	streams := tr.Key("QoorpAddedStreams1")
	if streams.Kind() != rscpdf.Array {
		t.Error("QoorpAddedStreams1 expected", rscpdf.Array, "got", streams.Kind())
	}
	checkStream(t, streams.Index(0), addedc)
}

func Test_pdfaddstream2(t *testing.T) {
	pdf1 := pdfFromSample(t)
	added1 := "some content"
	pdf1.AddStream([]byte(added1), rscpdf.ValueDict())
	added2 := "other content"
	pdf1.AddStream([]byte(added2), rscpdf.ValueDict())
	pdf2 := checkModification(t, pdf1, 882, 8)
	tr := pdf2.rscreader.Trailer()
	streams := tr.Key("QoorpAddedStreams1")
	if streams.Kind() != rscpdf.Array {
		t.Error("QoorpAddedStreams1 expected", rscpdf.Array, "got", streams.Kind())
	}
	checkStream(t, streams.Index(0), added1)
	checkStream(t, streams.Index(1), added2)
}

func Test_pdfreplacestream(t *testing.T) {
	pdf1 := pdfFromSample(t)
	newc := "BT\n/F1 20 Tf\n120 120 Td\n(Hello world) Tj\nET\n"
	compressedlength := 54
	objid := 5
	pdf1.replaceStream(objid, []byte(newc), rscpdf.ValueDict())
	pdf2 := checkModification(t, pdf1, 818, 6)
	found := pdf2.rscreader.Obj(objid)
	t.Skip("ReplaceStreams has a problem when the streams are mentioned in the trailer. Need work on rscreader.Obj()")
	if len(found) != 1 {
		t.Error("Obj expected 1 got", len(found))
	}
	v := found[0] // There is only one so it will be first.
	length := v.Key("Length")
	if length.Int64() != int64(compressedlength) {
		t.Error("Obj expected", compressedlength, "got", length.Int64())
	}
}

func Test_pdfaddfile(t *testing.T) {
	pdf1 := pdfFromSample(t)
	filename := "afile"
	addedc := "some content"
	pdf1.AddFile(filename, []byte(addedc), rscpdf.ValueDict())
	pdf2 := checkModification(t, pdf1, 871, 8)
	tr := pdf2.rscreader.Trailer()
	filespecs := tr.Key("QoorpAddedFiles1")
	if filespecs.Kind() != rscpdf.Array {
		t.Error("QoorpAddedFiles1 expected", rscpdf.Array, "got", filespecs.Kind())
	}
	checkFilespec(t, filespecs.Index(0), filename, addedc)
}

func Test_pdfaddfile2(t *testing.T) {
	pdf1 := pdfFromSample(t)
	filename1 := "afile"
	addedc1 := "some content"
	pdf1.AddFile(filename1, []byte(addedc1), rscpdf.ValueDict())
	filename2 := "bfile"
	addedc2 := "other content"
	pdf1.AddFile(filename2, []byte(addedc2), rscpdf.ValueDict())
	pdf2 := checkModification(t, pdf1, 1048, 10)
	b := bytes.Buffer{}
	_, err := pdf2.Write(&b)
	if err != nil {
		t.Error("Write", err)
	}
	us := b.String()
	ioutil.WriteFile("add.pdf", []byte(us), 0644)
	tr := pdf2.rscreader.Trailer()
	filespecs := tr.Key("QoorpAddedFiles1")
	if filespecs.Kind() != rscpdf.Array {
		t.Error("QoorpAddedFiles1 expected", rscpdf.Array, "got", filespecs.Kind())
	}
	checkFilespec(t, filespecs.Index(0), filename1, addedc1)
	checkFilespec(t, filespecs.Index(1), filename2, addedc2)
}

func pdfFromSample(t *testing.T) *PDF {
	content, err := ioutil.ReadFile("Sample.pdf")
	if err != nil {
		t.Error("ReadFile", err)
	}
	pdf1, err := NewPDF(content)
	if err != nil {
		t.Error("pdf", err)
	}
	return pdf1
}

func checkFilespec(t *testing.T, filespec rscpdf.Value, filename, content string) {
	if filespec.Kind() != rscpdf.Dict {
		t.Error("QoorpAddedFiles1 expected", rscpdf.Dict, "got", filespec.Kind())
	}
	f := filespec.Key("F")
	if f.RawString() != filename {
		t.Error("QoorpAddedFiles1 expected", filename, "got", f.String())
	}
	ef := filespec.Key("EF")
	if ef.Kind() != rscpdf.Dict {
		t.Error("EF expected", rscpdf.Dict, "got", ef.Kind())
	}
	stream := ef.Key("F")
	if stream.Kind() != rscpdf.Stream {
		t.Error("F expected", rscpdf.Stream, "got", stream.Kind())
	}
	qasr := stream.Reader()
	qasc, err := ioutil.ReadAll(qasr)
	if err != nil {
		t.Error("ReadAll got", err)
	}
	if string(qasc) != content {
		t.Error("ReadAll expected", content, "got", string(qasc))
	}
}

func checkModification(t *testing.T, original *PDF, xref, size int64) *PDF {
	b := bytes.Buffer{}
	_, err := original.Write(&b)
	if err != nil {
		t.Error("Write", err)
	}
	us := b.String()
	pdf2, err := NewPDF([]byte(us))
	if err != nil {
		t.Error("pdf2", err)
		t.Error(us)
	}
	// +X is comment before Qoorp additions in PDF.
	xrefComment := xref + 16
	if pdf2.rscreader.Startxref() != xrefComment {
		_, err := pdf2.Write(&b)
		if err != nil {
			t.Error("Write", err)
		}
		t.Error("startxref expected", xrefComment, "got", pdf2.rscreader.Startxref(), "in", b.String())
	}
	tr := pdf2.rscreader.Trailer()
	if tr.Key("Size").Int64() != size {
		t.Error("trailer expected", size, "got", tr.Key("Size").Int64(), "in", tr)
	}
	return pdf2
}

func checkStream(t *testing.T, stream rscpdf.Value, content string) {
	if stream.Kind() != rscpdf.Stream {
		t.Error("QoorpAddedStreams1 expected", rscpdf.Stream, "got", stream.Kind())
	}
	qasr := stream.Reader()
	qasc, err := ioutil.ReadAll(qasr)
	if err != nil {
		t.Error("QoorpAddedStream1 got", err)
	}
	if string(qasc) != content {
		t.Error("QoorpAddedStream1 expected", content, "got", string(qasc))
	}
}
