// Extract the GitHub integration's configuration from ServiceNow, so the
// native tables can be seeded before cutover.
//
// HOW TO RUN: paste into a background script window in the ServiceNow instance
// (System Definition > Scripts - Background) and run against PROD. Prints a
// report plus ready-to-run SQL. READ ONLY -- it contains no insert, update or
// delete, and can be run on production safely.
//
// WHAT IT IS FOR. Four system properties and one case field hold everything
// the integration needs. github.dispatch.config in particular is a JSON blob
// mapping an account name to the repository and credential its issues use;
// that becomes rows in account_github_repo. The rest is either diagnostic or
// superseded by the mapping table.

(function () {
  var out = [];
  function say(s) { out.push(s == null ? '' : String(s)); }
  function prop(name) {
    var gr = new GlideRecord('sys_properties');
    gr.addQuery('name', name);
    gr.query();
    return gr.next() ? gr.getValue('value') : null;
  }

  say('================ GitHub integration: ServiceNow configuration ================');
  say('instance : ' + gs.getProperty('instance_name'));
  say('extracted: ' + new GlideDateTime().getDisplayValue());
  say('');

  // ---------------------------------------------------------------- 1
  say('--- 1. github.dispatch.config (account -> repository + credential) ---');
  var raw = prop('github.dispatch.config');
  var mappings = [];
  if (!raw) {
    say('NOT SET. Either the property was renamed or routing is hardcoded;');
    say('check GitHubIssueContentProcessor before assuming there is nothing to seed.');
  } else {
    try {
      var cfg = JSON.parse(raw);
      Object.keys(cfg).forEach(function (accountName) {
        var e = cfg[accountName] || {};
        // Spellings vary between entries; take the first that is present
        // rather than assuming one shape.
        var owner = e.owner || e.org || e.organisation || e.organization || '';
        var repo  = e.repo  || e.repository || '';
        var cred  = e.credential || e.credential_ref || e.token || '';
        mappings.push({ account: accountName, owner: owner, repo: repo, cred: cred });
        say('  ' + accountName + '  ->  ' + owner + '/' + repo +
            (cred ? '   credential=' + cred : '   (NO CREDENTIAL)'));
      });
      say('');
      say('  ' + mappings.length + ' mapping(s).');
      // The credential may be a name OR an inlined secret. Say which, without
      // printing the value: this output gets pasted into tickets.
      var inlined = mappings.filter(function (m) { return m.cred && m.cred.length > 40; });
      if (inlined.length) {
        say('  WARNING: ' + inlined.length + ' entry/entries have a credential longer than 40');
        say('  characters, which suggests an inlined TOKEN rather than a reference.');
        say('  Treat this property as a secret, rotate those tokens at cutover, and');
        say('  put only the NAME in account_github_repo.credential_ref.');
      }
    } catch (e) {
      say('PRESENT BUT UNPARSEABLE: ' + e);
      say('Length ' + raw.length + ' chars. Inspect by hand; do not guess the shape.');
    }
  }
  say('');

  // ---------------------------------------------------------------- 2
  say('--- 2. git.integration.user-id (loop prevention) ---');
  var uid = prop('git.integration.user-id');
  if (!uid) {
    say('NOT SET. validate() drops every event when this is empty, so the');
    say('inbound integration is currently doing nothing.');
  } else {
    say('  value: ' + uid);
    say('  -> becomes GITHUB_INTEGRATION_LOGIN. Confirm this is the GitHub LOGIN');
    say('     of the bot account, not a ServiceNow sys_id.');
  }
  say('');

  // ---------------------------------------------------------------- 3
  say('--- 3. git.valid.org.list (diagnostic) ---');
  var orgs = prop('git.valid.org.list');
  if (!orgs) {
    say('  empty  -> the integration is LIVE.');
    say('  validate() calls .spit(",") on this value -- a typo for .split. While the');
    say('  property is empty that line never runs. Setting it would throw, the outer');
    say('  catch would swallow it, and the whole integration would silently stop.');
  } else {
    say('  value: ' + orgs);
    say('  NON-EMPTY -> the inbound integration has been DEAD since this was set,');
    say('  because of the .spit(",") typo. Check how much traffic it was really');
    say('  handling before treating its behaviour as a porting requirement.');
  }
  say('  (No equivalent is needed after cutover: account_github_repo IS the');
  say('   allow-list, so routing and permission cannot drift apart.)');
  say('');

  // ---------------------------------------------------------------- 4
  say('--- 4. scripted-wum.*-repo-name (superseded) ---');
  var wum = new GlideRecord('sys_properties');
  wum.addQuery('name', 'STARTSWITH', 'scripted-wum');
  wum.query();
  var wumCount = 0;
  while (wum.next()) { wumCount++; say('  ' + wum.getValue('name') + ' = ' + wum.getValue('value')); }
  if (!wumCount) say('  none found.');
  say('  -> superseded by account_github_repo. Listed so the cutover can confirm');
  say('     nothing else still reads them.');
  say('');

  // ---------------------------------------------------------------- 5
  // Which field holds the issue number is DISCOVERED, not assumed: guessing a
  // column name and finding nothing looks identical to there being no data.
  say('--- 5. the case field holding the issue number ---');
  var dict = new GlideRecord('sys_dictionary');
  dict.addQuery('name', 'sn_customerservice_case');
  dict.addQuery('element', 'CONTAINS', 'git');
  dict.query();
  var caseFields = [];
  while (dict.next()) {
    caseFields.push(dict.getValue('element'));
    say('  ' + dict.getValue('element') + '  (' + dict.getValue('internal_type') + ')  "' +
        dict.getValue('column_label') + '"');
  }
  dict = new GlideRecord('sys_dictionary');
  dict.addQuery('name', 'sn_customerservice_case');
  dict.addQuery('element', 'CONTAINS', 'issue');
  dict.query();
  while (dict.next()) {
    if (caseFields.indexOf(dict.getValue('element')) === -1) {
      caseFields.push(dict.getValue('element'));
      say('  ' + dict.getValue('element') + '  (' + dict.getValue('internal_type') + ')  "' +
          dict.getValue('column_label') + '"');
    }
  }
  if (!caseFields.length) say('  none found -- the link may live on a related table.');
  say('');

  // ---------------------------------------------------------------- 6
  say('--- 6. how much of this is actually used ---');
  var linked = new GlideAggregate('change_request');
  linked.addNotNullQuery('u_git_reference');
  linked.addAggregate('COUNT');
  linked.query();
  var total = linked.next() ? linked.getAggregate('COUNT') : 0;
  say('  change requests with u_git_reference set: ' + total);

  var recent = new GlideAggregate('change_request');
  recent.addNotNullQuery('u_git_reference');
  recent.addEncodedQuery('sys_updated_onRELATIVEGT@month@ago@6');
  recent.addAggregate('COUNT');
  recent.query();
  say('  ...updated in the last 6 months: ' + (recent.next() ? recent.getAggregate('COUNT') : 0));

  // Distinct repositories actually in use, which is the real scope of cutover.
  var repos = {};
  var cr = new GlideRecord('change_request');
  cr.addNotNullQuery('u_git_reference');
  cr.setLimit(5000);
  cr.query();
  while (cr.next()) {
    var m = String(cr.getValue('u_git_reference') || '').match(/github\.com\/([^\/]+)\/([^\/]+)\/issues\//);
    if (m) repos[m[1] + '/' + m[2]] = (repos[m[1] + '/' + m[2]] || 0) + 1;
  }
  say('  distinct repositories referenced (sample of up to 5000 rows):');
  Object.keys(repos).sort(function (a, b) { return repos[b] - repos[a]; })
    .forEach(function (r) { say('    ' + r + '  x' + repos[r]); });
  say('');

  // ---------------------------------------------------------------- 7
  // Emitted rather than executed: seeding runs against Postgres, and a human
  // should read the account names before any of it is applied.
  say('--- 7. SQL to seed account_github_repo ---');
  if (!mappings.length) {
    say('  (nothing to emit -- no mappings found)');
  } else {
    say('-- Review every account name below. A name that does not match an account');
    say('-- in Postgres inserts nothing and is reported by the final SELECT.');
    say('BEGIN;');
    mappings.forEach(function (m) {
      if (!m.owner || !m.repo) {
        say("-- SKIPPED " + m.account + ": incomplete entry (owner or repo missing)");
        return;
      }
      var acct = String(m.account).replace(/'/g, "''");
      say("INSERT INTO account_github_repo (id, created_by, updated_by, account_id, owner, repository, credential_ref)");
      say("SELECT gen_random_uuid(), 'sn-cutover', 'sn-cutover', a.id, '" +
          m.owner.replace(/'/g, "''") + "', '" + m.repo.replace(/'/g, "''") + "', " +
          (m.cred ? "'" + m.cred.replace(/'/g, "''") + "'" : 'NULL') +
          " FROM account a WHERE a.name = '" + acct + "'");
      say("ON CONFLICT (account_id) DO UPDATE SET owner=EXCLUDED.owner, repository=EXCLUDED.repository;");
    });
    say('-- Any account name that did not match:');
    say("SELECT v.name FROM (VALUES");
    say(mappings.map(function (m) {
      return "  ('" + String(m.account).replace(/'/g, "''") + "')";
    }).join(',\n'));
    say(") AS v(name) LEFT JOIN account a ON a.name = v.name WHERE a.id IS NULL;");
    say('COMMIT;');
  }

  say('');
  say('================ end ================');
  gs.print(out.join('\n'));
})();
