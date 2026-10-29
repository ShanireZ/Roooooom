import {defineConfig} from '@playwright/test';
import {mkdtempSync} from 'node:fs';
import {join} from 'node:path';
import {tmpdir} from 'node:os';
export default defineConfig({
 testDir:'./e2e', fullyParallel:false, workers:1, timeout:30000,
 globalSetup:'./e2e/global-setup.ts',
 use:{browserName:'chromium',viewport:{width:1440,height:960},trace:'retain-on-failure'},
 reporter:'list', outputDir:process.env.ROOOOOOM_EVIDENCE_DIR ?? mkdtempSync(join(tmpdir(),'roooooom-browser-evidence-')),
});
