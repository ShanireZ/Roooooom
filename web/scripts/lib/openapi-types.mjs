// Dependency-free OpenAPI 3.1 -> TypeScript declarations for web/src/api/contracts.generated.ts.
//
// This replaces openapi-typescript, which needs the TypeScript compiler JS API (TypeScript 7 no longer exports it).
// The output format is deliberately identical to what openapi-typescript 7.13.0 produced for this contract, so the
// client keeps its `components["schemas"][...]` surface. openapi-types.test.mjs pins that against the committed file
// and against reference texts produced by the old generator.
//
// Scope: exactly the OpenAPI constructs api/openapi.yaml uses. Anything else throws an OpenApiTypesError that names
// the JSON pointer of the offending node. Keywords that cannot change the TypeScript output are listed explicitly in
// IGNORED_SCHEMA_KEYWORDS (or are `x-` extensions and null-valued annotation markers); nothing is skipped silently.
//
// Supported schema keywords: $ref (local #/components/schemas/<name> only), type (string, number, integer, boolean,
// null, array, object), properties, required, items, additionalProperties, enum, const, oneOf, anyOf, allOf (inline
// members only), format, readOnly, description, default. Unsupported on purpose: nullable and type arrays (use
// anyOf with {"type":"null"}), tuples/prefixItems, patternProperties, discriminator, title/summary/deprecated/
// example(s), non-local $ref, webhooks, components other than schemas, parameter/response/header $refs.
// The input must be JSON-compatible YAML (JSON syntax), as api/openapi.yaml is.

export const HEADER = '// Generated from api/openapi.yaml. Do not edit.\n';

export class OpenApiTypesError extends Error {
 constructor(pointer, message) {
  super(pointer + ': ' + message);
  this.name = 'OpenApiTypesError';
  this.pointer = pointer;
 }
}
const fail = (pointer, message) => { throw new OpenApiTypesError(pointer, message); };

// Validation keywords and logic that TypeScript cannot express. They never change the generated types.
const NOT_EXPRESSIBLE = 'TypeScript cannot express this validation or logic';
export const IGNORED_SCHEMA_KEYWORDS = new Map([
 ...['minLength', 'maxLength', 'pattern', 'minimum', 'maximum', 'exclusiveMinimum', 'exclusiveMaximum', 'multipleOf',
  'minItems', 'maxItems', 'uniqueItems', 'minProperties', 'maxProperties', 'not', 'if', 'then', 'else'].map(k => [k, NOT_EXPRESSIBLE]),
 ['writeOnly', 'no read/write split in the generated types'],
]);
const SCHEMA_KEYWORDS = new Set(['$ref', 'type', 'properties', 'required', 'items', 'additionalProperties', 'enum', 'const', 'oneOf', 'anyOf', 'allOf', 'format', 'readOnly', 'description', 'default']);
const SCHEMA_TYPES = new Set(['string', 'number', 'integer', 'boolean', 'null', 'array', 'object']);
const STRUCTURAL = ['properties', 'additionalProperties', 'items', 'oneOf', 'anyOf', 'allOf'];
const METHODS = ['get', 'put', 'post', 'delete', 'options', 'head', 'patch', 'trace'];
const LOCATIONS = ['query', 'header', 'path', 'cookie'];
const IGNORED_DOCUMENT = new Set(['info', 'servers', 'security', 'tags', 'externalDocs']);
const IGNORED_COMPONENTS = new Set(['securitySchemes', 'examples', 'links', 'callbacks']);
const TYPE_SECTIONS = ['responses', 'parameters', 'requestBodies', 'headers', 'pathItems'];

const isObject = value => value !== null && typeof value === 'object' && !Array.isArray(value);
const escapePointer = key => key.replaceAll('~', '~0').replaceAll('/', '~1');
const child = (pointer, ...keys) => pointer + keys.map(key => '/' + escapePointer(String(key))).join('');

// Every object kind lists the keys it handles; others must be ignored deliberately or the generator throws.
function checkKeys(node, pointer, kind, handled, {ignored = new Set(), markers = false} = {}) {
 for (const key of Object.keys(node)) {
  if (handled.has(key) || ignored.has(key) || key.startsWith('x-')) continue;
  if (markers && node[key] === null) continue; // free-text annotation flag, e.g. "never a raw capability": null
  fail(child(pointer, key), `unsupported ${kind} keyword "${key}"`);
 }
}
function expectObject(node, pointer, what) {
 if (!isObject(node)) fail(pointer, what + ' must be an object');
 return node;
}

// ---- type tree ----------------------------------------------------------------------------------------------------
// Only the node kinds the output uses. `dedupe` marks primitives that collapse when repeated in a union/intersection.
const DEDUPED = new Set(['string', 'number', 'boolean', 'never']);
const keyword = name => ({kind: 'keyword', name, dedupe: DEDUPED.has(name) ? name : null});
const STRING = keyword('string'), NUMBER = keyword('number'), BOOLEAN = keyword('boolean'), NEVER = keyword('never'), UNKNOWN = keyword('unknown');
const NULL = {kind: 'literal', text: 'null', dedupe: 'null', primitive: true};
const literal = text => ({kind: 'literal', text, dedupe: null, primitive: false});
const named = (name, args = []) => ({kind: 'reference', name, args});
const indexed = (base, key) => ({kind: 'indexed', base, key});
const array = element => ({kind: 'array', element});
const object = members => ({kind: 'object', members});
const property = (name, type, {optional = false, readonly = false, doc = null} = {}) => ({kind: 'property', name, type, optional, readonly, doc});
const indexSignature = (parameter, type) => ({kind: 'index', parameter, type, doc: null});
const RECORD_NEVER = named('Record', [STRING, NEVER]);
const isPrimitive = type => (type.kind === 'keyword' && type.name !== 'unknown') || (type.kind === 'literal' && type.primitive);

function dedupe(types) {
 const seen = new Set();
 return types.filter(type => {
  if (!type.dedupe) return true;
  if (seen.has(type.dedupe)) return false;
  seen.add(type.dedupe);
  return true;
 });
}
const union = types => types.length === 0 ? NEVER : types.length === 1 ? types[0] : {kind: 'union', members: dedupe(types)};
const intersection = types => types.length === 0 ? NEVER : types.length === 1 ? types[0] : {kind: 'intersection', members: dedupe(types)};

// ---- printer (matches the TypeScript printer for these nodes) ---------------------------------------------------
const IDENTIFIER = /^[A-Za-z_$][A-Za-z_$0-9]*$/;
function quote(text, pointer) {
 if (!/^[\x20-\x7e]*$/.test(text)) fail(pointer, 'only printable ASCII is supported in names');
 return '"' + text.replaceAll('\\', '\\\\').replaceAll('"', '\\"') + '"';
}
function memberName(key, pointer) {
 if (key === '' || key.includes('#/')) fail(pointer, 'empty names and names containing "#/" are not supported');
 if (String(Number(key)) === key && key[0] !== '-') return key;
 return IDENTIFIER.test(key) ? key : quote(key, pointer);
}
const wrap = (type, text) => type.kind === 'union' || type.kind === 'intersection' ? '(' + text + ')' : text;
function printType(type, depth) {
 const pad = '    '.repeat(depth);
 switch (type.kind) {
  case 'keyword': return type.name;
  case 'literal': return type.text;
  case 'reference': return type.args.length ? `${type.name}<${type.args.map(arg => printType(arg, depth)).join(', ')}>` : type.name;
  case 'indexed': return `${printType(type.base, depth)}[${quote(type.key, '#')}]`;
  case 'array': return wrap(type.element, printType(type.element, depth)) + '[]';
  case 'union': return type.members.map(member => wrap(member, printType(member, depth))).join(' | ');
  case 'intersection': return type.members.map(member => wrap(member, printType(member, depth))).join(' & ');
  case 'object': return type.members.length ? '{\n' + type.members.map(member => printMember(member, depth + 1)).join('') + pad + '}' : '{}';
 }
}
function printComment(text, pad) {
 const lines = ('/*' + text + '*/').split('\n');
 return lines.map((line, index) => (index === 0 ? pad : pad + ' ') + line.trim() + '\n').join('');
}
function printMember(member, depth) {
 const pad = '    '.repeat(depth);
 const doc = member.doc ? printComment(member.doc, pad) : '';
 if (member.kind === 'index') return `${doc}${pad}[${member.parameter}: string]: ${printType(member.type, depth)};\n`;
 return `${doc}${pad}${member.readonly ? 'readonly ' : ''}${member.name}${member.optional ? '?' : ''}: ${printType(member.type, depth)};\n`;
}
const printInterface = (name, members) => `export interface ${name} {\n${members.map(member => printMember(member, 1)).join('')}}\n`;
const printAlias = (name, type) => `export type ${name} = ${printType(type, 0)};\n`;

// ---- JSDoc ------------------------------------------------------------------------------------------------------
const LINE_BREAK = /\r?\n/g;
function docOf(node, pointer) {
 const lines = [];
 if (node.format !== undefined) {
  if (typeof node.format !== 'string') fail(child(pointer, 'format'), 'format must be a string');
  if (node.format) lines.push('Format: ' + node.format);
 }
 if (node.description !== undefined) {
  if (typeof node.description !== 'string') fail(child(pointer, 'description'), 'description must be a string');
  if (node.description !== '') lines.push('@description ' + node.description.trim().replace(LINE_BREAK, '\n *     '));
 }
 if (node.default !== undefined) {
  const text = typeof node.default === 'object' ? JSON.stringify(node.default, null, 2) : node.default;
  lines.push('@default ' + String(text).trim().replace(LINE_BREAK, '\n *     '));
 }
 if ('const' in node) lines.push('@constant');
 if (node.enum) lines.push(`@enum {${typeof node.type === 'string' ? node.type : 'unknown'}}`);
 if (!lines.length) return null;
 const multiline = lines.some(line => line.includes('\n'));
 const text = (lines.length === 1 && !multiline ? `* ${lines[0]} ` : `*\n * ${lines.join('\n * ')}\n `).replaceAll('*/', '*\\/');
 if (/[\r\u2028\u2029]/.test(text)) fail(pointer, 'text contains an unsupported line terminator');
 return text;
}

// ---- generator --------------------------------------------------------------------------------------------------
export function generateDeclarations(spec) {
 expectObject(spec, '#', 'the OpenAPI document');
 checkKeys(spec, '#', 'document', new Set(['openapi', 'paths', 'components']), {ignored: IGNORED_DOCUMENT});
 if (typeof spec.openapi !== 'string' || !/^3\.1\.\d+$/.test(spec.openapi)) fail('#/openapi', 'only OpenAPI 3.1.x documents are supported');
 expectObject(spec.paths, '#/paths', 'paths');
 const components = expectObject(spec.components, '#/components', 'components');
 checkKeys(components, '#/components', 'components', new Set(['schemas', ...TYPE_SECTIONS]), {ignored: IGNORED_COMPONENTS});
 for (const section of TYPE_SECTIONS) {
  if (section in components && !(isObject(components[section]) && Object.keys(components[section]).length === 0)) fail(child('#/components', section), `components.${section} is not supported`);
 }
 const schemas = components.schemas === undefined ? {} : expectObject(components.schemas, '#/components/schemas', 'components.schemas');
 const operations = [];
 const operationIds = new Set();

 const reference = (node, pointer) => {
  const ref = node.$ref;
  const match = typeof ref === 'string' ? /^#\/components\/schemas\/([^/~%]+)$/.exec(ref) : null;
  if (!match) fail(child(pointer, '$ref'), 'only local #/components/schemas/<name> references are supported');
  if (!Object.hasOwn(schemas, match[1])) fail(child(pointer, '$ref'), `unresolved reference ${ref}`);
  return indexed(indexed(named('components'), 'schemas'), match[1]);
 };
 const literalOf = (value, pointer) => {
  if (typeof value === 'string') return literal(JSON.stringify(value));
  if (typeof value === 'number') return literal(value < 0 ? '-' + String(Math.abs(value)) : String(value));
  if (typeof value === 'boolean') return literal(String(value));
  if (value === null) return NULL;
  return fail(pointer, 'only string, number, boolean and null literals are supported');
 };

 // `opath` mirrors openapi-typescript's internal path string: it decides whether a property default makes the
 // property required (not inside parameters or request bodies), exactly as the old generator did.
 function schemaType(node, pointer, opath) {
  expectObject(node, pointer, 'schema');
  if (node.readOnly !== undefined && typeof node.readOnly !== 'boolean') fail(child(pointer, 'readOnly'), 'readOnly must be a boolean');
  if ('$ref' in node) {
   checkKeys(node, pointer, 'schema with $ref', new Set(['$ref', 'description', 'readOnly', 'default']), {ignored: IGNORED_SCHEMA_KEYWORDS, markers: true});
   return reference(node, pointer);
  }
  checkKeys(node, pointer, 'schema', SCHEMA_KEYWORDS, {ignored: IGNORED_SCHEMA_KEYWORDS, markers: true});
  if (node.type !== undefined && !SCHEMA_TYPES.has(node.type)) fail(child(pointer, 'type'), 'type must be one of ' + [...SCHEMA_TYPES].join(', '));
  if (node.required !== undefined && !(Array.isArray(node.required) && node.required.every(name => typeof name === 'string'))) fail(child(pointer, 'required'), 'required must be a list of names');
  for (const key of ['properties', 'additionalProperties']) {
   if (key in node && node.type !== undefined && node.type !== 'object') fail(child(pointer, key), `"${key}" requires type object`);
  }
  if ('items' in node && node.type !== 'array') fail(child(pointer, 'items'), '"items" requires type array');
  for (const key of ['oneOf', 'anyOf', 'allOf', 'enum']) {
   if (key in node && !Array.isArray(node[key])) fail(child(pointer, key), `${key} must be a list`);
  }

  if ('const' in node || 'enum' in node) {
   const keyName = 'const' in node ? 'const' : 'enum';
   if (keyName === 'enum' && node.type === 'object') fail(child(pointer, 'enum'), 'enum on object schemas is not supported');
   const conflict = STRUCTURAL.find(key => key in node);
   if (conflict) fail(pointer, `"${keyName}" cannot be combined with "${conflict}"`);
   if (keyName === 'const') {
    if (node.const === null) fail(child(pointer, 'const'), 'const null is not supported');
    return literalOf(node.const, child(pointer, 'const'));
   }
   return union(node.enum.map((value, index) => literalOf(value, child(pointer, 'enum', index))));
  }

  let core;
  if (node.type === 'null') core = NULL;
  else if (node.type === 'string') core = STRING;
  else if (node.type === 'number' || node.type === 'integer') core = NUMBER;
  else if (node.type === 'boolean') core = BOOLEAN;
  else if (node.type === 'array') core = arrayType(node, pointer, opath);
  else core = objectType(node, pointer, opath);

  let result;
  const allOf = (node.allOf ?? []).map((item, index) => allOfMember(item, child(pointer, 'allOf', index), opath, node.required));
  if (core || allOf.length) result = intersection([...(core ? [core] : []), ...(allOf.length ? [intersection(allOf)] : [])]);
  const anyOf = (node.anyOf ?? []).map((item, index) => schemaType(item, child(pointer, 'anyOf', index), child(opath, 'anyOf', index)));
  if (anyOf.length) result = union([...(result ? [result] : []), ...anyOf]);
  const oneOf = (node.oneOf ?? []).map((item, index) => schemaType(item, child(pointer, 'oneOf', index), child(opath, 'oneOf', index)));
  if (oneOf.length) result = oneOf.every(isPrimitive) ? union([...(result ? [result] : []), ...oneOf]) : intersection([...(result ? [result] : []), union(oneOf)]);
  return result ?? ('type' in node ? RECORD_NEVER : UNKNOWN);
 }

 function allOfMember(item, pointer, opath, required) {
  expectObject(item, pointer, 'schema');
  if ('$ref' in item) fail(pointer, 'allOf members must be inline schemas');
  if (item.required !== undefined && !Array.isArray(item.required)) fail(child(pointer, 'required'), 'required must be a list of names');
  return schemaType({...item, required: [...(required ?? []), ...(item.required ?? [])]}, pointer, opath);
 }

 function arrayType(node, pointer, opath) {
  if (Array.isArray(node.items)) fail(child(pointer, 'items'), 'tuple items are not supported');
  let element = UNKNOWN;
  if (node.items !== undefined) {
   const inner = schemaType(node.items, child(pointer, 'items'), opath);
   element = isObject(node.items) && node.items.type === 'array' ? array(inner) : inner;
  }
  return element.kind === 'array' ? element : array(element);
 }

 function objectType(node, pointer, opath) {
  const properties = node.properties === undefined ? {} : expectObject(node.properties, child(pointer, 'properties'), 'properties');
  const names = Object.keys(properties);
  const extra = node.additionalProperties;
  if (extra !== undefined && typeof extra !== 'boolean' && !isObject(extra)) fail(child(pointer, 'additionalProperties'), 'additionalProperties must be a boolean or a schema');
  if (!names.length && !extra) return undefined;
  const members = names.map(name => {
   const at = child(pointer, 'properties', name);
   const value = properties[name];
   if (!isObject(value)) fail(at, 'property schema must be an object');
   const hasDefault = value.default !== undefined && !/parameters|requestBody|requestBodies/.test(opath);
   const required = Array.isArray(node.required) && node.required.includes(name);
   const type = schemaType(value, at, child(opath, name));
   return property(memberName(name, at), type, {optional: !(required || hasDefault), readonly: value.readOnly === true, doc: docOf(value, at)});
  });
  const explicit = isObject(extra) && Object.keys(extra).length > 0;
  const implicit = extra === true || (isObject(extra) && Object.keys(extra).length === 0);
  const index = [...(explicit ? [schemaType(extra, child(pointer, 'additionalProperties'), opath)] : []), ...(implicit ? [UNKNOWN] : [])];
  if (!index.length) return object(members);
  return intersection([...(members.length ? [object(members)] : []), object([indexSignature('key', union(index))])]);
 }

 function parameters(list, pointer, opath) {
  const keyed = new Map();
  (list ?? []).forEach((parameter, index) => {
   const at = child(pointer, 'parameters', index);
   expectObject(parameter, at, 'parameter');
   checkKeys(parameter, at, 'parameter', new Set(['in', 'name', 'required', 'schema', 'description']));
   if (!LOCATIONS.includes(parameter.in)) fail(child(at, 'in'), 'in must be one of ' + LOCATIONS.join(', '));
   if (typeof parameter.name !== 'string') fail(child(at, 'name'), 'name must be a string');
   keyed.set(parameter.in + '-' + parameter.name, {parameter, at});
  });
  const groups = LOCATIONS.map(location => {
   const members = [...keyed.values()].filter(({parameter}) => parameter.in === location).map(({parameter, at}) => {
    const type = parameter.schema === undefined ? STRING : schemaType(parameter.schema, child(at, 'schema'), child(opath, 'parameters', location, parameter.name));
    return property(memberName(parameter.name, child(at, 'name')), type, {optional: location !== 'path' && parameter.required !== true, doc: docOf(parameter, at)});
   });
   return property(location, members.length ? object(members) : NEVER, {optional: members.every(member => member.optional)});
  });
  return property('parameters', object(groups));
 }

 function mediaTypes(content, pointer, opath) {
  expectObject(content, pointer, 'content');
  return Object.entries(content).map(([contentType, media]) => {
   const at = child(pointer, contentType);
   expectObject(media, at, 'media type');
   checkKeys(media, at, 'media type', new Set(['schema']));
   const type = media.schema === undefined ? UNKNOWN : schemaType(media.schema, child(at, 'schema'), child(opath, 'content', contentType));
   return property(memberName(contentType, at), type);
  });
 }

 function requestBody(body, pointer, opath) {
  if (body === undefined) return property('requestBody', NEVER, {optional: true});
  expectObject(body, pointer, 'requestBody');
  checkKeys(body, pointer, 'request body', new Set(['required', 'content', 'description']));
  const content = body.content === undefined ? [] : mediaTypes(body.content, child(pointer, 'content'), opath);
  const type = object([property('content', object(content.length ? content : [property(memberName('*/*', pointer), NEVER, {optional: true})]))]);
  return property('requestBody', type, {optional: !body.required, doc: docOf(body, pointer)});
 }

 function responses(map, pointer, opath) {
  const entries = Object.entries(expectObject(map ?? {}, pointer, 'responses')).map(([code, response]) => {
   const at = child(pointer, code);
   expectObject(response, at, 'response');
   checkKeys(response, at, 'response', new Set(['description', 'content']));
   const content = response.content === undefined ? [] : mediaTypes(response.content, child(at, 'content'), child(opath, code));
   const members = [
    property('headers', object([indexSignature('name', UNKNOWN)])),
    content.length ? property('content', object(content)) : property('content', NEVER, {optional: true}),
   ];
   return property(memberName(code, at), object(members), {doc: docOf(response, at)});
  });
  return property('responses', entries.length ? object(entries) : NEVER);
 }

 function operation(node, method, pointer, opath) {
  expectObject(node, pointer, 'operation');
  checkKeys(node, pointer, 'operation', new Set(['operationId', 'description', 'parameters', 'requestBody', 'responses']), {ignored: new Set(['security'])});
  const id = node.operationId;
  if (typeof id !== 'string' || id === '' || /[#/]/.test(id)) fail(child(pointer, 'operationId'), 'operationId must be a non-empty string without "#" or "/"');
  if (operationIds.has(id)) fail(child(pointer, 'operationId'), `duplicate operationId ${id}`);
  operationIds.add(id);
  if (node.parameters !== undefined && !Array.isArray(node.parameters)) fail(child(pointer, 'parameters'), 'parameters must be a list');
  operations.push(property(memberName(id, child(pointer, 'operationId')), object([
   parameters(node.parameters, pointer, opath),
   requestBody(node.requestBody, child(pointer, 'requestBody'), child(opath, 'requestBody')),
   responses(node.responses, child(pointer, 'responses'), child(opath, 'responses')),
  ])));
  return property(method, indexed(named('operations'), id), {doc: docOf(node, pointer)});
 }

 const paths = Object.entries(spec.paths).map(([url, item]) => {
  const pointer = child('#/paths', url);
  expectObject(item, pointer, 'path item');
  checkKeys(item, pointer, 'path item', new Set(METHODS));
  const members = [parameters([], pointer, child('#/paths', url))];
  for (const method of METHODS) {
   members.push(item[method] === undefined ? property(method, NEVER, {optional: true}) : operation(item[method], method, child(pointer, method), child('#/paths', url, method)));
  }
  return property(memberName(url, pointer), object(members));
 });
 const schemaMembers = Object.entries(schemas).map(([name, schema]) => {
  const pointer = child('#/components/schemas', name);
  return property(memberName(name, pointer), schemaType(schema, pointer, pointer), {doc: docOf(expectObject(schema, pointer, 'schema'), pointer)});
 });
 const sections = [property('schemas', schemaMembers.length ? object(schemaMembers) : NEVER), ...TYPE_SECTIONS.map(section => property(section, NEVER))];

 return (paths.length ? printInterface('paths', paths) : printAlias('paths', RECORD_NEVER))
  + printAlias('webhooks', RECORD_NEVER)
  + printInterface('components', sections)
  + printAlias('$defs', RECORD_NEVER)
  + (operations.length ? printInterface('operations', operations) : printAlias('operations', RECORD_NEVER));
}

export const renderContractsFile = spec => HEADER + generateDeclarations(spec);
