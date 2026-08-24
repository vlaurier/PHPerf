<?php
/**
 * PHPerf — wrapper de profilage CLI générique.
 *
 * Exécute un scénario PHP sous XHProf et écrit le profil au format JSON
 * consommé par phperf / phperf-ci / l'interface web.
 *
 * Usage :
 *   php phperf-profile.php [options] <scenario.php> [-- <args pour le scénario>]
 *
 * Options :
 *   --output=FICHIER  Fichier de sortie (défaut : "-" = stdout).
 *   --no-cpu          Ne pas collecter la CPU (XHPROF_FLAGS_CPU).
 *   --no-memory       Ne pas collecter la mémoire (mu/pmu forcés à 0).
 *   --no-builtins     Ignorer les fonctions natives — déconseillé : plusieurs
 *                     règles PHPerf ciblent précisément des builtins.
 *
 * Prérequis : ext-xhprof (pecl install xhprof && extension=xhprof.so).
 *
 * Codes de sortie : 0 succès · 1 le scénario a échoué (profil partiel
 * écrit quand même) · 2 erreur d'environnement ou d'usage.
 */

declare(strict_types=1);

if (PHP_SAPI !== 'cli') {
    fwrite(STDERR, "[phperf] à exécuter en CLI uniquement\n");
    exit(2);
}

if (!function_exists('xhprof_enable')) {
    fwrite(STDERR, <<<TXT
        [phperf] extension xhprof absente.
        Installez-la puis réessayez :
            pecl install xhprof
            echo 'extension=xhprof.so' > \$(php -ini | grep 'Scan this dir' | cut -d: -f2)/xhprof.ini

        TXT);
    exit(2);
}

$options = getopt('', ['output::', 'no-cpu', 'no-memory', 'no-builtins']);

$flags = 0;
if (!isset($options['no-cpu'])) {
    $flags |= XHPROF_FLAGS_CPU;
}
if (!isset($options['no-memory'])) {
    $flags |= XHPROF_FLAGS_MEMORY;
}
if (isset($options['no-builtins'])) {
    $flags |= XHPROF_FLAGS_NO_BUILTINS;
}

// Le scénario est le premier argument positionnel ; tout ce qui suit "--"
// lui est transmis via $argv.
$sep = array_search('--', $argv, true);
$tail = $sep === false ? [] : array_slice($argv, $sep + 1);
$positional = array_values(array_filter(
    array_slice($argv, 1, $sep === false ? null : $sep - 1),
    static fn (string $a): bool => !str_starts_with($a, '--')
));

if ($positional === []) {
    fwrite(STDERR, "Usage: php phperf-profile.php [options] <scenario.php> [-- <args>]\n");
    exit(2);
}

$scenario = realpath($positional[0]);
if ($scenario === false || !is_file($scenario)) {
    fwrite(STDERR, "[phperf] scénario introuvable : {$positional[0]}\n");
    exit(2);
}

// Rend visible au scénario uniquement ses propres arguments.
$argv = [$positional[0], ...$tail];
$argc = count($argv);

xhprof_enable($flags);

$failed = false;
try {
    require $scenario;
} catch (Throwable $e) {
    // Un profil partiel reste exploitable : on le sauvegarde avant de sortir.
    fwrite(STDERR, '[phperf] scénario interrompu : ' . $e->getMessage() . "\n");
    $failed = true;
}

$data = xhprof_disable();

// Normalisation : entiers garantis, champs mémoire présents même sans
// XHPROF_FLAGS_MEMORY, et racine "main()" toujours définie.
$maxWt = $maxCpu = $maxMu = $maxPmu = 0;
foreach ($data as $key => $row) {
    $data[$key] = [
        'ct' => (int) ($row['ct'] ?? 0),
        'wt' => (int) ($row['wt'] ?? 0),
        'cpu' => (int) ($row['cpu'] ?? 0),
        'mu' => (int) ($row['mu'] ?? 0),
        'pmu' => (int) ($row['pmu'] ?? 0),
    ];
    if ($key === 'main()') {
        continue;
    }
    $maxWt = max($maxWt, $data[$key]['wt']);
    $maxCpu = max($maxCpu, $data[$key]['cpu']);
    $maxMu = max($maxMu, $data[$key]['mu']);
    $maxPmu = max($maxPmu, $data[$key]['pmu']);
}
if (!isset($data['main()'])) {
    $data = ['main()' => ['ct' => 1, 'wt' => $maxWt, 'cpu' => $maxCpu,
                          'mu' => $maxMu, 'pmu' => $maxPmu]] + $data;
}

$json = json_encode($data, JSON_PRETTY_PRINT | JSON_UNESCAPED_SLASHES | JSON_UNESCAPED_UNICODE);
if ($json === false) {
    fwrite(STDERR, '[phperf] encodage JSON impossible : ' . json_last_error_msg() . "\n");
    exit(2);
}

$output = $options['output'] ?? '-';
if ($output === '-') {
    fwrite(STDOUT, $json . "\n");
} elseif (@file_put_contents($output, $json . "\n") === false) {
    fwrite(STDERR, "[phperf] écriture impossible dans $output\n");
    exit(2);
} else {
    fwrite(STDERR, "[phperf] profil écrit : $output (" . count($data) . " entrées)\n");
}

exit($failed ? 1 : 0);
