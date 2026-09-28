use std::path::PathBuf;

#[test]
fn phase_one_loads_all_reference_recordings() {
    let corpus_root = PathBuf::from(env!("CARGO_MANIFEST_DIR")).join("../../reference/silk");
    let report = silk_corpus::load_corpus(&corpus_root).expect("load reference corpus");

    println!("{report}");
    assert_eq!(report.loaded, 200);
    assert_eq!(report.passing, 0);
    assert_eq!(report.failing, 200);
}
