# N: number of times to run each experiment
# define variable N below
N=1

# Arguments: 
# $1: experiment directory
deploy_the_system() {
    local experiment_dir=$1
    echo "Deploying the system for experiment $(basename $experiment_dir)"
    ansible-playbook -i $experiment_dir/hosts ../ansible/deploy.yaml -e "experiment_path=$experiment_dir" -vv
    if [ $? -eq 0 ]; then
        echo "System deployed successfully"
    else
        echo "System deployment failed"
    fi
}

# Arguments:
# $1: experiment directory path
run_experiment_N_times() {
    local experiment_dir_path=$1

    for i in $(seq 1 $N)
    do
        echo "Running experiment $i for $(basename $experiment_dir_path)"
        ansible-playbook -i $experiment_dir_path/hosts ../ansible/experiment.yaml -e "experiment_output_path=$experiment_dir_path/experiment_$i.txt" -vv
        if [ $? -eq 0 ]; then
            echo "Experiment $i ran successfully"
        else
            echo "Experiment $i failed"
        fi
    done
}

# Arguments:
# $1: experiment directory path
# Returns:
# 0 if all experiments exist
# 1 if any experiment does not exist
do_all_experiments_exist() {
    local experiment_dir=$1
    for i in $(seq 1 $N)
    do
        if [ ! -f $experiment_dir/experiment_$i.txt ]; then
            return 1
        fi
    done
    return 0
}

run_experiments() {
    # for experiment_group in $(pwd)/*
    # do
    # for experiment in /Users/aminst/Documents/oblishard/experiments/path_oram_stash_experiments_v2/*
    # do
    experiment="/Users/aminst/Documents/oblishard/experiments/path_oram_stash_experiments_v2/A1K10000"
        echo $experiment
        if do_all_experiments_exist $experiment -eq 0 ; then
            echo "All experiments already exist for $(basename $experiment)"
            continue
        fi
        if [ -d $experiment ]; then
            deploy_the_system $experiment
            run_experiment_N_times $experiment
        fi
    # done
    # done
}

run_experiments