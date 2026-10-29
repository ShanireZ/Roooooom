// Tests for the in-repo OpenAPI -> TypeScript generator (openapi-types.mjs) and its api-types.mjs entry.
// openapi-types.cases.json holds the expected text for each focused shape. It was produced by openapi-typescript 7.13.0
// (the generator this module replaced) so the shapes stay byte-compatible with what the web client was written against.
import {spawnSync} from 'node:child_process';
import {mkdtempSync, readFileSync, rmSync, writeFileSync} from 'node:fs';
import {tmpdir} from 'node:os';
import {join} from 'node:path';
import {fileURLToPath} from 'node:url';
import {afterAll, describe, expect, it} from 'vitest';
import {IGNORED_SCHEMA_KEYWORDS, OpenApiTypesError, generateDeclarations, renderContractsFile} from './openapi-types.mjs';

const path = relative => fileURLToPath(new URL(relative, import.meta.url));
const entry = path('../api-types.mjs');
const specPath = path('../../../api/openapi.yaml');
const committedPath = path('../../src/api/contracts.generated.ts');
const shapeCases = JSON.parse(readFileSync(path('./openapi-types.cases.json'), 'utf8'));
const temp = mkdtempSync(join(tmpdir(), 'roooooom-openapi-types-'));
afterAll(() => rmSync(temp, {recursive: true, force: true}));

const lf = text => text.replaceAll('\r\n', '\n');
const clone = value => structuredClone(value);
const repoSpec = () => JSON.parse(readFileSync(specPath, 'utf8'));
const widgetDoc = () => ({openapi: '3.1.0', info: {title: 't', version: '1'}, paths: {}, components: {schemas: {Widget: {type: 'object', properties: {id: {type: 'string'}}}}}});
const opDoc = () => ({...widgetDoc(), paths: {'/w': {get: {operationId: 'getW', parameters: [{in: 'query', name: 'q', schema: {type: 'string'}}], responses: {200: {description: 'ok', content: {'application/json': {schema: {$ref: '#/components/schemas/Widget'}}}}}}}}});
const writeSpec = (name, spec) => { const file = join(temp, name); writeFileSync(file, JSON.stringify(spec)); return file; };
const runEntry = (...args) => spawnSync(process.execPath, [entry, ...args], {encoding: 'utf8', windowsHide: true});

describe('committed contracts', () => {
 it('equal the generator output for api/openapi.yaml, byte for byte (line endings aside)', () => {
  const committed = lf(readFileSync(committedPath, 'utf8'));
  const generated = renderContractsFile(repoSpec());
  expect(generated.length).toBe(committed.length);
  expect(generated === committed).toBe(true);
 });
});

describe('api-types.mjs entry', () => {
 it('--check passes for the committed spec and types', () => {
  const result = runEntry('--check');
  expect(result.stderr).toBe('');
  expect(result.status).toBe(0);
 });
 it('--check fails when the spec drifts from the committed types', () => {
  const spec = repoSpec();
  spec.components.schemas.Mutation.properties.driftProbe = {type: 'string'};
  const result = runEntry('--check', '--spec', writeSpec('drift.json', spec));
  expect(result.status).not.toBe(0);
  expect(result.stderr).toContain('API types drift');
 });
 it('--check fails with the JSON pointer of an unsupported keyword', () => {
  const spec = repoSpec();
  spec.components.schemas.Mutation.properties.driftProbe = {type: 'string', nullable: true};
  const result = runEntry('--check', '--spec', writeSpec('unsupported.json', spec));
  expect(result.status).not.toBe(0);
  expect(result.stderr).toContain('#/components/schemas/Mutation/properties/driftProbe/nullable');
 });
 it('rejects a spec that is not JSON-compatible YAML', () => {
  const file = join(temp, 'real.yaml');
  writeFileSync(file, 'openapi: 3.1.0\ninfo:\n  title: yaml\n');
  const result = runEntry('--check', '--spec', file);
  expect(result.status).not.toBe(0);
  expect(result.stderr).toContain('JSON-compatible YAML');
 });
 it('rejects unknown arguments', () => {
  const result = runEntry('--chek');
  expect(result.status).not.toBe(0);
  expect(result.stderr).toContain('Unknown argument');
 });
 it('generates the expected file for --spec and --out, then --check accepts it', () => {
  const spec = opDoc();
  const specFile = writeSpec('generate.json', spec);
  const out = join(temp, 'generated.ts');
  const generate = runEntry('--spec', specFile, '--out', out);
  expect(generate.status).toBe(0);
  expect(readFileSync(out, 'utf8')).toBe(renderContractsFile(spec));
  expect(runEntry('--check', '--spec', specFile, '--out', out).status).toBe(0);
  writeFileSync(out, readFileSync(out, 'utf8').replace('id?: string', 'id: string'));
  expect(runEntry('--check', '--spec', specFile, '--out', out).status).not.toBe(0);
 });
 it('--check accepts a CRLF checkout of the same content', () => {
  const spec = opDoc();
  const specFile = writeSpec('crlf.json', spec);
  const out = join(temp, 'crlf.ts');
  writeFileSync(out, renderContractsFile(spec).replaceAll('\n', '\r\n'));
  expect(runEntry('--check', '--spec', specFile, '--out', out).status).toBe(0);
 });
});

describe('shape cases', () => {
 it.each(shapeCases.map(c => [c.name, c]))('%s', (_name, c) => {
  expect(generateDeclarations(c.spec)).toBe(c.expected.join('\n') + '\n');
 });
});

describe('focused shapes', () => {
 const schemas = (entries) => ({...widgetDoc(), components: {schemas: entries}});
 const members = text => text.split('\n').slice(4, text.split('\n').indexOf('    responses: never;') - 1).join('\n');
 it('marks optional, required and readonly members', () => {
  const text = generateDeclarations(schemas({T: {type: 'object', required: ['a'], properties: {a: {type: 'string'}, b: {type: 'string'}, c: {type: 'string', readOnly: true}}}}));
  expect(members(text)).toBe(['        T: {', '            a: string;', '            b?: string;', '            readonly c?: string;', '        };'].join('\n'));
 });
 it('renders a union with null and a union of primitives', () => {
  const text = generateDeclarations(schemas({A: {type: 'string'}, N: {anyOf: [{$ref: '#/components/schemas/A'}, {type: 'null'}]}, P: {oneOf: [{type: 'string'}, {type: 'integer'}]}}));
  expect(members(text)).toContain('        N: components["schemas"]["A"] | null;');
  expect(members(text)).toContain('        P: string | number;');
 });
 it('renders additionalProperties as an index signature, unknown or nothing', () => {
  const text = generateDeclarations(schemas({Typed: {type: 'object', additionalProperties: {type: 'integer'}}, Open: {type: 'object', additionalProperties: true}, Shut: {type: 'object', additionalProperties: false}}));
  expect(members(text)).toBe(['        Typed: {', '            [key: string]: number;', '        };', '        Open: {', '            [key: string]: unknown;', '        };', '        Shut: Record<string, never>;'].join('\n'));
 });
 it('renders enum and const as literal unions with JSDoc markers', () => {
  const text = generateDeclarations(schemas({E: {type: 'string', enum: ['x', 'y']}, K: {const: 1}}));
  expect(members(text)).toBe(['        /** @enum {string} */', '        E: "x" | "y";', '        /** @constant */', '        K: 1;'].join('\n'));
 });
 it('parenthesizes union and intersection element types', () => {
  const text = generateDeclarations(schemas({L: {type: 'array', items: {oneOf: [{type: 'string'}, {type: 'object', properties: {a: {type: 'string'}}}]}}}));
  expect(members(text)).toContain('(string | {');
  expect(members(text)).toContain('})[];');
 });
});

describe('ignored keywords', () => {
 const samples = {
  minLength: 1, maxLength: 9, pattern: '^a', minimum: 0, maximum: 9, exclusiveMinimum: 0, exclusiveMaximum: 10, multipleOf: 1,
  minItems: 1, maxItems: 9, uniqueItems: true, minProperties: 1, maxProperties: 9,
  not: {required: ['a']}, if: {required: ['a']}, then: {required: ['a']}, else: {required: ['a']}, writeOnly: true,
 };
 it('has a sample for every documented ignored keyword and nothing else', () => {
  expect(Object.keys(samples).sort()).toEqual([...IGNORED_SCHEMA_KEYWORDS.keys()].sort());
 });
 it.each(Object.entries(samples))('%s does not change the output', (keyword, value) => {
  const plain = widgetDoc();
  const annotated = widgetDoc();
  annotated.components.schemas.Widget[keyword] = value;
  annotated.components.schemas.Widget.properties.id[keyword] = value;
  expect(generateDeclarations(annotated)).toBe(generateDeclarations(plain));
 });
 it('treats x- extensions and null-valued annotation markers as notes', () => {
  const plain = widgetDoc();
  const annotated = widgetDoc();
  annotated.components.schemas.Widget.properties.id['x-note'] = 'anything';
  annotated.components.schemas.Widget.properties.id['never a raw capability'] = null;
  annotated['x-extension'] = {anything: true};
  expect(generateDeclarations(annotated)).toBe(generateDeclarations(plain));
 });
});

describe('unsupported input fails loudly with a JSON pointer', () => {
 const refTo = name => ({$ref: '#/components/schemas/' + name});
 const W = '#/components/schemas/Widget';
 const O = '#/paths/~1w/get';
 const cases = [
  ['nullable', widgetDoc, d => { d.components.schemas.Widget.properties.id.nullable = true; }, W + '/properties/id/nullable'],
  ['type array', widgetDoc, d => { d.components.schemas.Widget.properties.id.type = ['string', 'null']; }, W + '/properties/id/type'],
  ['unknown type', widgetDoc, d => { d.components.schemas.Widget.properties.id.type = 'text'; }, W + '/properties/id/type'],
  ['discriminator', widgetDoc, d => { d.components.schemas.Widget.discriminator = {propertyName: 'id'}; }, W + '/discriminator'],
  ['prefixItems', widgetDoc, d => { d.components.schemas.Widget.properties.id = {type: 'array', prefixItems: [{type: 'string'}]}; }, W + '/properties/id/prefixItems'],
  ['tuple items', widgetDoc, d => { d.components.schemas.Widget.properties.id = {type: 'array', items: [{type: 'string'}]}; }, W + '/properties/id/items'],
  ['patternProperties', widgetDoc, d => { d.components.schemas.Widget.patternProperties = {'^a': {type: 'string'}}; }, W + '/patternProperties'],
  ['title', widgetDoc, d => { d.components.schemas.Widget.title = 'Widget'; }, W + '/title'],
  ['unknown keyword with a value', widgetDoc, d => { d.components.schemas.Widget.mystery = 1; }, W + '/mystery'],
  ['boolean schema', widgetDoc, d => { d.components.schemas.Widget.properties.id = true; }, W + '/properties/id'],
  ['properties on a string', widgetDoc, d => { d.components.schemas.Widget.properties.id.properties = {}; }, W + '/properties/id/properties'],
  ['items on an object', widgetDoc, d => { d.components.schemas.Widget.items = {type: 'string'}; }, W + '/items'],
  ['const null', widgetDoc, d => { d.components.schemas.Widget.properties.id = {const: null}; }, W + '/properties/id/const'],
  ['const with structure', widgetDoc, d => { d.components.schemas.Widget.properties.id = {const: 'a', properties: {}}; }, W + '/properties/id'],
  ['object enum', widgetDoc, d => { d.components.schemas.Widget.enum = ['a']; }, W + '/enum'],
  ['object enum value', widgetDoc, d => { d.components.schemas.Widget.properties.id = {type: 'string', enum: [{a: 1}]}; }, W + '/properties/id/enum/0'],
  ['required not a list', widgetDoc, d => { d.components.schemas.Widget.required = 'id'; }, W + '/required'],
  ['allOf $ref item', widgetDoc, d => { d.components.schemas.Widget.allOf = [refTo('Widget')]; }, W + '/allOf/0'],
  ['external $ref', widgetDoc, d => { d.components.schemas.Widget.properties.id = {$ref: 'other.yaml#/X'}; }, W + '/properties/id/$ref'],
  ['unresolved $ref', widgetDoc, d => { d.components.schemas.Widget.properties.id = refTo('Missing'); }, W + '/properties/id/$ref'],
  ['non-schema $ref', widgetDoc, d => { d.components.schemas.Widget.properties.id = {$ref: '#/components/parameters/P'}; }, W + '/properties/id/$ref'],
  ['$ref sibling', widgetDoc, d => { d.components.schemas.Widget.properties.id = {...refTo('Widget'), type: 'string'}; }, W + '/properties/id/type'],
  ['example keyword', widgetDoc, d => { d.components.schemas.Widget.properties.id.example = 'a'; }, W + '/properties/id/example'],
  ['non-ASCII property name', widgetDoc, d => { d.components.schemas.Widget.properties['café'] = {type: 'string'}; }, W + '/properties/café'],
  ['components.responses', widgetDoc, d => { d.components.responses = {R: {description: 'x'}}; }, '#/components/responses'],
  ['components.parameters', widgetDoc, d => { d.components.parameters = {P: {in: 'query', name: 'p'}}; }, '#/components/parameters'],
  ['webhooks', widgetDoc, d => { d.webhooks = {w: {}}; }, '#/webhooks'],
  ['top-level $defs', widgetDoc, d => { d.$defs = {}; }, '#/$defs'],
  ['top-level keyword', widgetDoc, d => { d.jsonSchemaDialect = 'x'; }, '#/jsonSchemaDialect'],
  ['openapi version', widgetDoc, d => { d.openapi = '3.0.3'; }, '#/openapi'],
  ['missing paths', widgetDoc, d => { delete d.paths; }, '#/paths'],
  ['path item $ref', opDoc, d => { d.paths['/w'] = {$ref: '#/x'}; }, '#/paths/~1w/$ref'],
  ['path-level parameters', opDoc, d => { d.paths['/w'].parameters = []; }, '#/paths/~1w/parameters'],
  ['operation tags', opDoc, d => { d.paths['/w'].get.tags = ['a']; }, O + '/tags'],
  ['operation summary', opDoc, d => { d.paths['/w'].get.summary = 'a'; }, O + '/summary'],
  ['missing operationId', opDoc, d => { delete d.paths['/w'].get.operationId; }, O + '/operationId'],
  ['operationId with a slash', opDoc, d => { d.paths['/w'].get.operationId = 'a/b'; }, O + '/operationId'],
  ['duplicate operationId', opDoc, d => { d.paths['/v'] = clone(d.paths['/w']); }, '#/paths/~1v/get/operationId'],
  ['parameter $ref', opDoc, d => { d.paths['/w'].get.parameters = [{$ref: '#/components/parameters/P'}]; }, O + '/parameters/0/$ref'],
  ['parameter style', opDoc, d => { d.paths['/w'].get.parameters[0].style = 'form'; }, O + '/parameters/0/style'],
  ['parameter location', opDoc, d => { d.paths['/w'].get.parameters[0].in = 'body'; }, O + '/parameters/0/in'],
  ['response $ref', opDoc, d => { d.paths['/w'].get.responses[200] = {$ref: '#/x'}; }, O + '/responses/200/$ref'],
  ['response headers', opDoc, d => { d.paths['/w'].get.responses[200].headers = {}; }, O + '/responses/200/headers'],
  ['media type example', opDoc, d => { d.paths['/w'].get.responses[200].content['application/json'].example = {}; }, O + '/responses/200/content/application~1json/example'],
  ['request body $ref', opDoc, d => { d.paths['/w'].get.requestBody = {$ref: '#/x'}; }, O + '/requestBody/$ref'],
 ];
 it.each(cases)('%s', (_name, make, mutate, pointer) => {
  const doc = make();
  mutate(doc);
  let error;
  try { generateDeclarations(doc); } catch (caught) { error = caught; }
  expect(error).toBeInstanceOf(OpenApiTypesError);
  expect(error.message.startsWith(pointer + ': ')).toBe(true);
 });
});
