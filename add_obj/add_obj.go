package add_obj

import (
	"bytes"
	"compress/zlib"
	"errors"
	"fmt"
	"io"
	"log"

	rscpdf "github.com/qoorp/pdf"
)

// The keys we use in the PDF trailer.
// The files are not visible as normal attachments.
const (
	QoorpAddedFiles   = "QoorpAddedFiles1"
	QoorpAddedStreams = "QoorpAddedStreams1"
	// Not used yet.
	QoorpReplacedStreams = "QoorpReplacedStreams1"
)

// PDF is existing PDF (in original) that we want to add PDF objects too.
type PDF struct {
	addobjs     []*Obj
	original    []byte
	nobj        int64
	replaceobjs []*Obj
	rscreader   *rscpdf.Reader
}

// NewPDF creates *PDF from contents.
func NewPDF(original []byte) (*PDF, error) {
	r, err := rscpdf.NewReader(bytes.NewReader(original), int64(len(original)))
	if err != nil {
		return &PDF{}, err
	}
	t := r.Trailer()
	value := t.Key("Size")
	size := value.Int64()
	if size < 1 {
		return &PDF{}, errors.New("too small Size")
	}
	return &PDF{original: original, rscreader: r, nobj: size}, nil
}

// AddFile adds a file attachment with name filename and contents from contents.
func (p *PDF) AddFile(filename string, content []byte, dict rscpdf.Value) error {
	streamDict := rscpdf.ValueDict()
	streamDict.SetMapIndex("Type", rscpdf.ValueName("EmbeddedFile"))
	err := p.AddStream(content, streamDict)
	if err != nil {
		return err
	}
	// The stream added above.
	stream := p.addobjs[len(p.addobjs)-1]
	filespec, err := filespecObj(int(p.nobj), 0, stream.obj, stream.generation, filename, dict)
	if err != nil {
		return err
	}
	p.addobjs = append(p.addobjs, filespec)
	p.nobj = p.nobj + 1
	return nil
}

// AddStream adds a Stream, created from contents.
func (p *PDF) AddStream(content []byte, dict rscpdf.Value) error {
	obj, err := streamObj(int(p.nobj), 0, content, dict)
	if err != nil {
		return err
	}
	p.addobjs = append(p.addobjs, obj)
	p.nobj = p.nobj + 1
	return nil
}

// replaceStream replaces an existing Obj with contents.
// Not usable yet..
// Need better rscreader.Obj() before allowing them into trailer.
// Objs are found twice if mentioned in trailer?
func (p *PDF) replaceStream(obj int, content []byte, dict rscpdf.Value) error {
	if obj == 0 {
		return errors.New("Can not replace magic object 0")
	}
	_, gen := p.rscreader.Xref(obj)
	if gen == 65535 {
		return errors.New("Can not replace none-existing object")
	}
	// Do not increase generation. We are replacing the existing one.
	o, err := streamObj(obj, gen, content, dict)
	if err != nil {
		return err
	}
	p.replaceobjs = append(p.replaceobjs, o)
	return nil
}

// Write PDF to writer.
func (p *PDF) Write(w io.Writer) (int, error) {
	var result int
	if len(p.replaceobjs) == 0 && len(p.addobjs) == 0 {
		return w.Write(p.original)
	}
	result, err := w.Write(p.original)
	if err != nil {
		return result, err
	}
	n, err := p.writeObjs(w)
	result = result + n
	if err != nil {
		return result, err
	}
	startxref := result
	n, err = p.writeXref(w)
	result = result + n
	if err != nil {
		return result, err
	}
	n, err = p.writeTrailer(w)
	result = result + n
	if err != nil {
		return result, err
	}
	n, err = fmt.Fprintf(w, "startxref\n%v\n%%%%EOF\n", startxref)
	result = result + n
	return result, err
}

// Obj is PDF object
type Obj struct {
	dict       rscpdf.Value
	obj        int
	generation int
	offset     int
	stream     *[]byte
}

//
// Internal functions
//

func filespecObj(obj, generation, fobj, fgeneration int, filename string, dict rscpdf.Value) (*Obj, error) {
	dict.SetMapIndex("Type", rscpdf.ValueName("Filespec"))
	dict.SetMapIndex("F", rscpdf.ValueString(filename))
	ef := rscpdf.ValueDict()
	ef.SetMapIndex("F", rscpdf.ValueObj(fobj, fgeneration))
	dict.SetMapIndex("EF", ef)
	return &Obj{obj: obj, generation: generation, dict: dict}, nil
}

func streamObj(obj, generation int, content []byte, dict rscpdf.Value) (*Obj, error) {
	var z []byte // contents after zlib
	null := rscpdf.Value{}
	value := dict.Key("Filter")
	// If there is no Filter, we add one.
	if value == null {
		filter := rscpdf.ValueName("FlateDecode")
		dict.SetMapIndex("Filter", filter)
		var b bytes.Buffer
		w := zlib.NewWriter(&b)
		w.Write(content)
		w.Close()
		z = b.Bytes()
	} else {
		z = content
	}
	value = dict.Key("Length")
	// If there is no Length, we add one.
	if value == null {
		length := rscpdf.ValueInt64(int64(len(z)))
		dict.SetMapIndex("Length", length)
	}
	return &Obj{obj: obj, generation: generation, dict: dict, stream: &z}, nil
}

func (o *Obj) write(w io.Writer) (int, error) {
	result, err := fmt.Fprintf(w, "%v %v obj\n%v\n", o.obj, o.generation, o.dict.Ustring())
	if err != nil {
		return result, err
	}
	if o.stream != nil {
		n, err := fmt.Fprintf(w, "stream\n%s\nendstream\n", *o.stream)
		result = result + n
		if err != nil {
			return result, err
		}
	}
	n, err := fmt.Fprintln(w, "endobj")
	result = result + n
	if err != nil {
		return result, err
	}
	return result, nil
}

// Write all objects, and save the offsets from start to each object.
func (p *PDF) writeObjs(w io.Writer) (int, error) {
	result, err := fmt.Fprintf(w, "%% Qoorp additions\n")
	if err != nil {
		return result, err
	}
	for _, r := range p.replaceobjs {
		r.offset = result
		n, err := r.write(w)
		result = result + n
		if err != nil {
			return result, err
		}
	}
	for _, a := range p.addobjs {
		a.offset = result
		n, err := a.write(w)
		result = result + n
		if err != nil {
			return result, err
		}
	}
	return result, nil
}

func (p *PDF) writeTrailer(w io.Writer) (int, error) {
	t := p.rscreader.Trailer()
	size := rscpdf.ValueInt64(int64(p.nobj))
	t.SetMapIndex("Size", size)
	prev := rscpdf.ValueInt64(int64(p.rscreader.Startxref()))
	t.SetMapIndex("Prev", prev)
	if len(p.addobjs) > 0 {
		writeTrailerAdd(p.addobjs, t)
	}
	if len(p.replaceobjs) > 0 {
		writeTrailerReplace(p.replaceobjs, t)
	}
	return fmt.Fprintf(w, "trailer\n%v\n", t.Ustring())
}

func writeTrailerAdd(objs []*Obj, t rscpdf.Value) {
	fs := rscpdf.ValueArray()
	ss := rscpdf.ValueArray()
	for _, obj := range objs {
		fs, ss = writeTrailerAppend(fs, ss, obj)
	}
	t.SetMapIndex(QoorpAddedFiles, fs)
	t.SetMapIndex(QoorpAddedStreams, ss)
}

func writeTrailerAppend(fs, ss rscpdf.Value, obj *Obj) (rscpdf.Value, rscpdf.Value) {
	null := rscpdf.Value{}
	v := rscpdf.ValueObj(obj.obj, obj.generation)
	if v != null {
		if obj.stream != nil {
			ss = ss.Append(v)
		} else {
			fs = fs.Append(v)
		}
	} else {
		log.Println("writeTrailerAppend failed for", obj)
	}
	return fs, ss
}

func writeTrailerReplace(objs []*Obj, t rscpdf.Value) {
	rs := rscpdf.ValueArray()
	null := rscpdf.Value{}
	for _, obj := range objs {
		v := rscpdf.ValueObj(obj.obj, obj.generation)
		if v != null {
			rs = rs.Append(v)
		} else {
			log.Println("writeTrailerReplace failed for", obj)
		}
	}
	t.SetMapIndex(QoorpReplacedStreams, rs)
}

func (p *PDF) writeXref(w io.Writer) (int, error) {
	result, err := fmt.Fprintln(w, "xref")
	if err != nil {
		return result, err
	}
	bodylen := len(p.original)
	for _, r := range p.replaceobjs {
		_, gen := p.rscreader.Xref(r.obj)
		if gen >= 65535 {
			log.Println("Warning: Object does not exist in Xref:", r.obj)
			continue
		}
		n, err := fmt.Fprintf(w, "%d 1\n%010d %05d n \n", r.obj, r.offset+bodylen, r.generation)
		result = result + n
		if err != nil {
			return result, err
		}
	}
	n := len(p.addobjs)
	if n > 0 {
		n, err = fmt.Fprintln(w, p.addobjs[0].obj, n)
		result = result + n
		if err != nil {
			return result, err
		}
	}
	for _, a := range p.addobjs {
		n, err := fmt.Fprintf(w, "%010d %05d n \n", a.offset+bodylen, a.generation)
		result = result + n
		if err != nil {
			return result, err
		}
	}
	return result, nil
}
